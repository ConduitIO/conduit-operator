package conduit

import (
	"context"

	"github.com/conduitio/conduit/pkg/plugin"
)

type PluginRegistry interface {
	Register(ctx context.Context, path string) (plugin.FullName, error)
}
