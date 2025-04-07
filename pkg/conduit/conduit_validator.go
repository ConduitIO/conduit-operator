//go:generate mockgen --build_flags=--mod=mod -source=./conduit_validator.go -destination=mock/http_client_mock.go -package=mock

package conduit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/conduitio/conduit-commons/config"
	sdk "github.com/conduitio/conduit-connector-sdk"
	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	pconfig "github.com/conduitio/conduit/pkg/provisioning/config"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	baseURL      = "https://conduit.io/connectors/github.com"
	connectorURL = "https://conduit.io/connectors.json"
	conduitOrg   = "conduitio"
)

var HTTPClient httpClient = http.DefaultClient

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var _ ValidatorService = (*Validator)(nil)

type PluginInfo struct {
	Name    string
	Org     string
	URL     string
	Version string
}

type Validator struct {
	Log           logr.Logger
	ConnectorList map[string]PluginInfo
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

func NewValidator(ctx context.Context, log logr.Logger) *Validator {
	plugins, err := connectorList(ctx)
	if err != nil {
		log.Error(err, "unable to construct connector validation list %w")
		plugins = nil
	}

	return &Validator{
		Log:           log,
		ConnectorList: plugins,
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

func (v *Validator) ValidateProcessorPlugin(p *v1alpha.ConduitProcessor, fp *field.Path) *field.Error {
	if p.Plugin == "" {
		return field.Required(fp.Child("plugin"), "plugin cannot be empty")
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
	if len(v.ConnectorList) == 0 {
		v.Log.Info("connector list is empty, skipping parameter validation")
		return nil
	}

	plugin, err := v.filterConnector(c.PluginName)
	if err != nil {
		v.Log.Error(err, "error filtering connector from list")
		return nil
	}
	if plugin == nil {
		v.Log.Error(err, fmt.Sprintf("connector %s not listed in cache", c.Name))
		return nil
	}

	spec, err := v.fetchYAMLSpec(ctx, c, plugin)
	if err != nil {
		v.Log.Error(err, fmt.Sprintf("getting plugin parameters for connector %s", c.Name))
		return nil
	}

	settings := make(map[string]string)
	for _, v := range c.Settings {
		settings[v.Name] = v.Value
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
// Returns the github organization and the transformed connector name
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
		return nil, err
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

	for _, p := range v.ConnectorList {
		if strings.Contains(p.Name, pluginName) {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("no matching plugin found in cache")
}
