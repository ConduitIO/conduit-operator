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
	baseURL    = "https://conduit.io/connectors/github.com"
	conduitOrg = "conduitio"
)

var HTTPClient httpClient = http.DefaultClient

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var _ ValidatorService = (*Validator)(nil)

type Validator struct {
	Log logr.Logger
}

func NewValidator(log logr.Logger) *Validator {
	return &Validator{
		Log: log,
	}
}

func (v *Validator) ValidateConnector(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	validations := []func(*v1alpha.ConduitConnector, *field.Path) *field.Error{
		v.validateConnectorPlugin,
		v.validateConnectorPluginType,
		v.validateConnectorParameters,
	}

	for _, v := range validations {
		if err := v(c, fp); err != nil {
			return err
		}
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

func (v *Validator) validateConnectorParameters(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if !(c.Type == pconfig.TypeSource || c.Type == pconfig.TypeDestination) {
		return field.InternalError(fp.Child("parameter"), fmt.Errorf("connector type %s is not recognized", c.Type))
	}
	spec, err := v.fetchYAMLSpec(c)
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
func (v *Validator) fetchYAMLSpec(c *v1alpha.ConduitConnector) (func() sdk.Specification, error) {
	ctx := context.Background()
	emptySpecFn := func() sdk.Specification { return sdk.Specification{} }

	cn, org := formatPluginName(c.PluginName)
	ver, err := v.pluginVersion(ctx, c.PluginVersion, cn, org)
	if err != nil {
		return emptySpecFn, fmt.Errorf("getting plugin version with error %w", err)
	}
	connectorURL := fmt.Sprintf("%s/%s/%s@%s/connector.yaml", baseURL, org, cn, ver)

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
// "conduit-connector-connectorName" to match the name in github
// Returns the github organization and the transformed connector name
func formatPluginName(pn string) (string, string) {
	parts := strings.Split(strings.TrimPrefix(strings.ToLower(pn), "github.com/"), "/")

	org, name := "", ""
	switch len(parts) {
	case 1:
		// handle transforming "builtin:connector" to desired format
		trimmedName := strings.TrimPrefix(parts[0], "builtin:")
		if slices.Contains(
			BuiltinConnectors,
			trimmedName,
		) {
			return fmt.Sprintf("conduit-connector-%s", trimmedName), conduitOrg
		}

		name = trimmedName
	case 2:
		org, name = parts[0], parts[1]
	}

	return org, name
}

// pluginVersion will either return the ver in the parameter or parse a version "latest"
// into the latest version number
func (v *Validator) pluginVersion(ctx context.Context, ver string, n string, org string) (string, error) {
	if ver == "latest" {
		pluginURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", org, n)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pluginURL, nil)
		if err != nil {
			return "", err
		}

		resp, err := HTTPClient.Do(req)
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("getting latest release with status code: %d", resp.StatusCode)
		}
		defer resp.Body.Close()

		var rel struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			return "", err
		}

		v.Log.Info("Connector plugin %s set to version 'latest', version %s found", n, rel.TagName)
		return rel.TagName, nil
	}

	return ver, nil
}
