package conduit

import (
	"context"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit/pkg/schemaregistry"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type ValidatorService interface {
	ValidateConnector(ctx context.Context, c *v1alpha.ConduitConnector, fp *field.Path) *field.Error
	ValidateProcessorPlugin(p *v1alpha.ConduitProcessor, fp *field.Path) *field.Error
	ValidateProcessorSchema(ctx context.Context, p *v1alpha.ConduitProcessor, sr schemaregistry.Registry, fp *field.Path) *field.Error
}
