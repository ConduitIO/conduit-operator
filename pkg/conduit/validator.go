package conduit

import (
	"context"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit/pkg/plugin/processor/standalone"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type ValidatorService interface {
	ValidateConnector(ctx context.Context, c *v1alpha.ConduitConnector, fp *field.Path) *field.Error
	ValidateProcessor(ctx context.Context, p *v1alpha.ConduitProcessor, r *standalone.Registry, fp *field.Path) *field.Error
}
