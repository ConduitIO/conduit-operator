//go:generate mockgen --build_flags=--mod=mod -source=./http_client.go -destination=mock/http_client_mock.go -package=mock
//go:generate mockgen --build_flags=--mod=mod -source=./plugin_registry.go -destination=mock/registry_mock.go -package=mock

package conduit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/conduitio/conduit-commons/config"
	sdk "github.com/conduitio/conduit-connector-sdk"
	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit/pkg/plugin"
	"github.com/conduitio/conduit/pkg/plugin/processor/standalone"
	pconfig "github.com/conduitio/conduit/pkg/provisioning/config"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	baseURL      = "https://conduit.io/connectors/github.com"
	connectorURL = "https://conduit.io/connectors.json"
	conduitOrg   = "conduitio"
)

var HTTPClient httpClient = http.DefaultClient

var _ PluginRegistry = (*standalone.Registry)(nil)

var _ ValidatorService = (*Validator)(nil)

type PluginInfo struct {
	Name    string
	Org     string
	URL     string
	Version string
}

type Validator struct {
	client        client.Client
	log           logr.Logger
	connectorList map[string]PluginInfo
}

type ConnectorInfo struct {
	Name     string     `json:"name_with_owner"`
	URL      string     `json:"url"`
	Releases []Releases `json:"releases"`
}

type Releases struct {
	Name   string `json:"name"`
	Latest bool   `json:"is_latest"`
}

func NewValidator(ctx context.Context, cl client.Client, log logr.Logger) *Validator {
	plugins, err := connectorList(ctx)
	if err != nil {
		log.Error(err, "unable to construct connector validation list %w")
		plugins = nil
	}

	return &Validator{
		client:        cl,
		log:           log,
		connectorList: plugins,
	}
}

func (v *Validator) ValidateConnector(ctx context.Context, c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	validations := []func(*v1alpha.ConduitConnector, *field.Path) *field.Error{
		v.validateConnectorPlugin,
		v.validateConnectorPluginType,
	}

	for _, v := range validations {
		if err := v(c, fp); err != nil {
			return err
		}
	}

	if err := v.validateConnectorParameters(ctx, c, fp); err != nil {
		return err
	}

	return nil
}

func (v *Validator) ValidateProcessor(ctx context.Context, p *v1alpha.ConduitProcessor, reg PluginRegistry, fp *field.Path) *field.Error {
	if err := v.validateProcessorPlugin(p, fp); err != nil {
		return err
	}

	if err := v.validateStandaloneProcessor(ctx, p, reg, fp); err != nil {
		return err
	}

	return nil
}

func (v *Validator) validateProcessorPlugin(p *v1alpha.ConduitProcessor, fp *field.Path) *field.Error {
	if p.Plugin == "" {
		return field.Required(fp.Child("plugin"), "plugin cannot be empty")
	}
	return nil
}

func (v *Validator) validateStandaloneProcessor(ctx context.Context, p *v1alpha.ConduitProcessor, reg PluginRegistry, fp *field.Path) *field.Error {
	if p.ProcessorURL == "" {
		return nil
	}

	file, cleanup, err := pluginWASM(ctx, p.ProcessorURL)
	if err != nil {
		return field.InternalError(fp.Child("standalone"), fmt.Errorf("failed to save wasm to file: %w", err))
	}
	if cleanup != nil {
		defer cleanup()
	}

	_, err = reg.Register(ctx, file)
	if err != nil {
		if errors.Is(err, plugin.ErrPluginAlreadyRegistered) {
			return nil
		}
		return field.InternalError(fp.Child("standalone"), fmt.Errorf("failed to register: %w", err))
	}

	return nil
}

func (v *Validator) validateConnectorPlugin(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if err := ValidatePlugin(c.Plugin); err != nil {
		return field.Invalid(fp.Child("plugin"), c.Plugin, err.Error())
	}
	return nil
}

func (v *Validator) validateConnectorPluginType(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if err := ValidatePluginType(c.Type); err != nil {
		return field.Invalid(fp.Child("type"), c.Type, err.Error())
	}
	return nil
}

func (v *Validator) validateConnectorParameters(ctx context.Context, c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if !(c.Type == pconfig.TypeSource || c.Type == pconfig.TypeDestination) {
		return field.InternalError(fp.Child("parameter"), fmt.Errorf("connector type %s is not recognized", c.Type))
	}
	if len(v.connectorList) == 0 {
		v.log.Info("connector list is empty, skipping parameter validation")
		return nil
	}

	plugin, err := v.filterConnector(c.PluginName)
	if err != nil {
		v.log.Error(err, "error filtering connector from list")
		return nil
	}
	if plugin == nil {
		v.log.Error(err, fmt.Sprintf("connector %s not listed in cache", c.Name))
		return nil
	}

	spec, err := v.fetchYAMLSpec(ctx, c, plugin)
	if err != nil {
		v.log.Error(err, fmt.Sprintf("getting plugin parameters for connector %s", c.Name))
		return nil
	}

	settings := make(map[string]string)
	errs := make(map[string]error)
	for _, setting := range c.Settings {
		val, err := v.valueOrSecret(ctx, setting)
		if err != nil {
			errs[setting.Name] = err
		}
		settings[setting.Name] = val
	}
	// aggregate all settings errors into one
	if len(errs) > 0 {
		errMsg := fmt.Sprintf("getting secrets for connector %s: ", c.Name)
		for setting, err := range errs {
			errMsg += fmt.Sprintf("{ setting: %s, err: %s} ", setting, err)
		}

		return field.InternalError(fp.Child("parameter"), errors.New(errMsg))
	}

	config := config.Config(settings)

	if c.Type == pconfig.TypeSource {
		err = config.Validate(spec().SourceParams)
		if err != nil {
			return field.Invalid(fp.Child("parameter"), c.Type, err.Error())
		}
	} else if c.Type == pconfig.TypeDestination {
		err = config.Validate(spec().DestinationParams)
		if err != nil {
			return field.Invalid(fp.Child("parameter"), c.Type, err.Error())
		}
	}

	return nil
}

// fetchYAMLSpec makes a call to conduit.io to get the connector.yaml
// for the appropriate plugin. If this call fails, an empty spec is returned.
func (v *Validator) fetchYAMLSpec(ctx context.Context, c *v1alpha.ConduitConnector, plugin *PluginInfo) (func() sdk.Specification, error) {
	emptySpecFn := func() sdk.Specification { return sdk.Specification{} }
	if plugin.Version == "" {
		return emptySpecFn, fmt.Errorf("no version found for connector %s", plugin.Name)
	}

	connectorURL := fmt.Sprintf("%s/%s/%s@%s/connector.yaml", baseURL, plugin.Org, plugin.Name, plugin.Version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, connectorURL, nil)
	if err != nil {
		return emptySpecFn, fmt.Errorf("creating the http request %w", err)
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return emptySpecFn, fmt.Errorf("getting yaml from cache with error %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return emptySpecFn, fmt.Errorf("getting yaml, status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return emptySpecFn, fmt.Errorf("reading response body %w", err)
	}

	return sdk.YAMLSpecification(string(body), c.PluginVersion), nil
}

// formatPluginName converts the plugin name into the format
// "conduit-connector-connectorName"
// Returns the transformed connector name
func formatPluginName(pn string) (string, error) {
	parts := strings.Split(strings.TrimPrefix(strings.ToLower(pn), "github.com/"), "/")

	switch len(parts) {
	case 1:
		// handle transforming "builtin:connector" to desired format
		trimmedName := strings.TrimPrefix(parts[0], "builtin:")
		if slices.Contains(
			BuiltinConnectors,
			trimmedName,
		) {
			return fmt.Sprintf("conduit-connector-%s", trimmedName), nil
		}
	case 2:
		return parts[1], nil
	}

	return "", nil
}

// pluginWASM gets the processor WASM from the specified URL and saves to a temp
// file.
// Returns the filename of the created file and a cleanup function fo the file
func pluginWASM(ctx context.Context, processorURL string) (string, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, processorURL, nil)
	if err != nil {
		return "", nil, err
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("non-sucessful status code while getting processor WASM, status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	file, err := os.CreateTemp("", "proc-*.wasm")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		file.Close()
	}()

	if _, err = io.Copy(file, resp.Body); err != nil {
		return "", nil, err
	}

	return file.Name(), func() {
		if err := os.Remove(file.Name()); err != nil {
			log.Println(err)
		}
	}, nil
}

// connectorList constructs a dictionary of connectors with information for
// use by the validator. Skips any connectors with improper name formmatting or
// are not in allowed orgs.
func connectorList(ctx context.Context) (map[string]PluginInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, connectorURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getting connector list, status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	// parse response into a dictionary to look up values
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var connectors []ConnectorInfo
	err = json.Unmarshal(body, &connectors)
	if err != nil {
		return nil, err
	}

	plugins := make(map[string]PluginInfo)
	for _, c := range connectors {
		parts := strings.Split(strings.ToLower(c.Name), "/")
		if len(parts) != 2 {
			continue
		}

		org := parts[0]
		if !slices.Contains(allowedGitHubOrgs, fmt.Sprintf("%s/", org)) {
			continue
		}

		plugin := PluginInfo{
			Name: parts[1],
			Org:  org,
			URL:  c.URL,
		}
		for _, rel := range c.Releases {
			if rel.Latest {
				plugin.Version = rel.Name
				break
			}
		}

		plugins[plugin.Name] = plugin
	}

	return plugins, nil
}

func (v *Validator) filterConnector(n string) (*PluginInfo, error) {
	pluginName, err := formatPluginName(n)
	if err != nil {
		return nil, err
	}

	for _, p := range v.connectorList {
		if strings.Contains(p.Name, pluginName) {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("no matching plugin found in cache")
}

func (v *Validator) valueOrSecret(ctx context.Context, settings v1alpha.SettingsVar) (string, error) {
	if settings.SecretRef == nil {
		return settings.Value, nil
	}

	var secret corev1.Secret
	if err := v.client.Get(
		ctx,
		client.ObjectKey{
			Name: settings.SecretRef.Name,
		},
		&secret,
	); err != nil {
		return "", fmt.Errorf("failed to get %q secret: %w", settings.SecretRef.Name, err)
	}

	val, ok := secret.Data[settings.SecretRef.Key]
	if !ok {
		return "", fmt.Errorf("secret ref key %s does not exist", settings.SecretRef.Key)
	}

	return string(val), nil
}
