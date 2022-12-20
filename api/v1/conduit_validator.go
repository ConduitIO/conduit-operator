package v1

import (
	"github.com/conduitio-labs/conduit-operator/pkg/conduit"
)

var connectorValidators = []func(*ConduitConnector) error{
	ValidatePlugin,
	ValidatePluginType,
}

func ValidatePlugin(c *ConduitConnector) error {
	return conduit.ValidatePlugin(c.Plugin)
}

func ValidatePluginType(c *ConduitConnector) error {
	return conduit.ValidatePluginType(c.Type)
}
