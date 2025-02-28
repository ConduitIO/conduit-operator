package v1alpha

import (
	"fmt"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/conduit"
)

var connectorValidators = []func(*v1alpha.ConduitConnector) error{
	validateConnectorPlugin,
	validateConnectorPluginType,
}

var processorValidators = []func(*v1alpha.ConduitProcessor) error{
	validateProcessorPlugin,
}

func validateConnectorPlugin(c *v1alpha.ConduitConnector) error {
	return conduit.ValidatePlugin(c.Plugin)
}

func validateConnectorPluginType(c *v1alpha.ConduitConnector) error {
	return conduit.ValidatePluginType(c.Type)
}

func validateProcessorPlugin(p *v1alpha.ConduitProcessor) error {
	if p.Plugin == "" {
		return fmt.Errorf("plugin cannot be empty")
	}

	return nil
}
