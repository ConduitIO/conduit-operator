package v1alpha

import (
	"fmt"

	"github.com/conduitio/conduit-operator/pkg/conduit"
)

var connectorValidators = []func(*ConduitConnector) error{
	validateConnectorPlugin,
	validateConnectorPluginType,
}

var processorValidators = []func(*ConduitProcessor) error{
	validateProcessorPlugin,
}

func validateConnectorPlugin(c *ConduitConnector) error {
	return conduit.ValidatePlugin(c.Plugin)
}

func validateConnectorPluginType(c *ConduitConnector) error {
	return conduit.ValidatePluginType(c.Type)
}

func validateProcessorPlugin(p *ConduitProcessor) error {
	if p.Plugin == "" {
		return fmt.Errorf("plugin cannot be empty")
	}

	return nil
}
