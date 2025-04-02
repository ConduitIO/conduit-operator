package conduit

import (
	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type ValidatorService interface {
	ValidateConnector(c *v1alpha.ConduitConnector, fp *field.Path) *field.Error
	ValidateProcessorPlugin(p *v1alpha.ConduitProcessor, fp *field.Path) *field.Error
}
