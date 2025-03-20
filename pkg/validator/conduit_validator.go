//go:generate mockgen --build_flags=--mod=mod -source=./conduit_validator.go -destination=mock/http_client_mock.go -package=mock

package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/conduitio/conduit-commons/config"
	sdk "github.com/conduitio/conduit-connector-sdk"
	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/conduit"
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

var _ Validator = (*ConduitValidator)(nil)

type ConduitValidator struct{}

func NewConduitValidator() *ConduitValidator {
	return &ConduitValidator{}
}

func (v *ConduitValidator) ValidateConnector(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
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

func (v *ConduitValidator) validateConnectorPlugin(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if err := conduit.ValidatePlugin(c.Plugin); err != nil {
		return field.Invalid(fp.Child("plugin"), c.Plugin, err.Error())
	}
	return nil
}

func (v *ConduitValidator) validateConnectorPluginType(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if err := conduit.ValidatePluginType(c.Type); err != nil {
		return field.Invalid(fp.Child("type"), c.Type, err.Error())
	}
	return nil
}

func (v *ConduitValidator) ValidateProcessorPlugin(p *v1alpha.ConduitProcessor, fp *field.Path) *field.Error {
	if p.Plugin == "" {
		return field.Required(fp.Child("plugin"), "plugin cannot be empty")
	}
	return nil
}

func (v *ConduitValidator) validateConnectorParameters(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if !(c.Type == "source" || c.Type == "destination") {
		return field.InternalError(fp.Child("parameter"), fmt.Errorf("connector type %s is not recognized", c.Type))
	}
	spec, err := getPluginParameters(c)
	if err != nil {
		// TODO log an error
		return nil
	}

	settings := make(map[string]string)
	for _, v := range c.Settings {
		settings[v.Name] = v.Value
	}

	config := config.Config(settings)

	if c.Type == "source" {
		err = config.Validate(spec().SourceParams)
		if err != nil {
			return field.Invalid(fp.Child("parameter"), c.Type, err.Error())
		}
	} else if c.Type == "destination" {
		err = config.Validate(spec().DestinationParams)
		if err != nil {
			return field.Invalid(fp.Child("parameter"), c.Type, err.Error())
		}
	}

	return nil
}

func getPluginParameters(c *v1alpha.ConduitConnector) (func() sdk.Specification, error) {
	body, err := getCachedYaml(c)
	if err != nil {
		return func() sdk.Specification { return sdk.Specification{} }, err
	}

	return sdk.YAMLSpecification(body, c.PluginVersion), nil
}

// getCachedYaml makes a call to conduit.io to get the connector.yaml
// for the appropriate plugin
func getCachedYaml(c *v1alpha.ConduitConnector) (string, error) {
	ctx := context.Background()

	cn, org := getConnectorInfo(c.PluginName)
	ver, err := getPluginVersion(ctx, c.PluginVersion, cn, org)
	if err != nil {
		return "", fmt.Errorf("getting plugin version with error %w", err)
	}
	connectorURL := fmt.Sprintf("%s/%s/%s@%s/connector.yaml", baseURL, org, cn, ver)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, connectorURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating the http request %w", err)
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("getting yaml from cache with error %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getting yaml, status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body %w", err)
	}
	return string(body), nil
}

// getConnectorInfo converts the connector string into the format
// "conduit-connector-connectorName" to match the name in github
// Returns the github organization and the transformed connector name
func getConnectorInfo(pn string) (string, string) {
	trimmedName := strings.ToLower(pn)
	trimmedName = strings.TrimPrefix(trimmedName, "github.com/")

	prefixPattern := "[A-Za-z0-9_.-]/[A-Za-z0-9_.-]"
	hasPrefix, err := regexp.MatchString(prefixPattern, trimmedName)
	if err != nil {
		return "", ""
	}
	if hasPrefix {
		info := strings.Split(trimmedName, "/")
		return info[0], info[1]
	}

	// handle transforming "builtin:connector" to desired format
	trimmedName = strings.TrimPrefix(trimmedName, "builtin:")
	if slices.Contains(
		conduit.BuiltinConnectors,
		trimmedName,
	) {
		return fmt.Sprintf("conduit-connector-%s", trimmedName), conduitOrg
	}

	return "", pn
}

// getPluginVersion will either return the ver in the parameter or parse a version "latest"
// into the latest version number
func getPluginVersion(ctx context.Context, ver string, n string, org string) (string, error) {
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

		return rel.TagName, nil
	}

	return ver, nil
}
