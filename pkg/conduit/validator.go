package conduit

import (
	"context"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type ValidatorService interface {
	ValidateConnector(ctx context.Context, c *v1alpha.ConduitConnector, fp *field.Path) *field.Error
	ValidateProcessorPlugin(p *v1alpha.ConduitProcessor, fp *field.Path) *field.Error
	ValidateProcessorSchema(p *v1alpha.ConduitProcessor, fp *field.Path) *field.Error
}
