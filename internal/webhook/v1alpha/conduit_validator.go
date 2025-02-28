package v1alpha

import (
	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/conduit"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

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
