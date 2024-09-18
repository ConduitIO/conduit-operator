package v1alpha

import (
	"github.com/conduitio/conduit-operator/pkg/conduit"
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
