//go:generate mockgen --build_flags=--mod=mod -source=./conduit_validator.go -destination=mock/http_client_mock.go -package=mock

package v1alpha

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/conduitio/conduit-commons/config"
	sdk "github.com/conduitio/conduit-connector-sdk"
	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/conduit"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const baseURL = "https://conduit.io/connectors/github.com/ConduitIO"

var httpClient HTTPClient = http.DefaultClient

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var connectorValidators = []func(*v1alpha.ConduitConnector, *field.Path) *field.Error{
	validateConnectorPlugin,
	validateConnectorPluginType,
}

func validateConnectorPlugin(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if err := conduit.ValidatePlugin(c.Plugin); err != nil {
		return field.Invalid(fp.Child("plugin"), c.Plugin, err.Error())
	}
	return nil
}

func validateConnectorPluginType(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if err := conduit.ValidatePluginType(c.Type); err != nil {
		return field.Invalid(fp.Child("type"), c.Type, err.Error())
	}
	return nil
}

func validateProcessorPlugin(p *v1alpha.ConduitProcessor, fp *field.Path) *field.Error {
	if p.Plugin == "" {
		return field.Required(fp.Child("plugin"), "plugin cannot be empty")
	}
	return nil
}

func validateConnectorParameters(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error {
	if !(c.Type == "source" || c.Type == "destination") {
		return field.InternalError(fp.Child("parameter"), fmt.Errorf("connector type %s is not recognized", c.Type))
	}
	spec, err := getPluginParameters(c)
	if err != nil {
		return field.InternalError(fp.Child("parameter"), fmt.Errorf("failed getting plugin params from cache with error %w", err))
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
			return field.Invalid(fp.Child("parameter"), c.Type, err.Error()) // this will print out source since the connector is source. Problem?
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

func getCachedYaml(c *v1alpha.ConduitConnector) (string, error) {
	connectorURL := fmt.Sprintf("%s/%s%%40%s/connector.yaml", baseURL, c.PluginName, c.PluginVersion)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, connectorURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating the http request %w", err)
	}

	resp, err := httpClient.Do(req)
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
