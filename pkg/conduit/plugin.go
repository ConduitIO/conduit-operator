package conduit

import (
	"fmt"
	"slices"
	"strings"
)

var allowedGitHubOrgs = []string{
	"conduitio/",
	"conduitio-labs/",
	"meroxa/",
}

var forbiddenConnectors = []string{
	"github.com/conduitio/conduit-connector-file",
	"conduitio/conduit-connector-file",
}

var BuiltinConnectors = []string{
	KafkaPlugin,
	GeneratorPlugin,
	S3Plugin,
	PostgresPlugin,
	LogPlugin,
}

const (
	KafkaPlugin     = "kafka"
	GeneratorPlugin = "generator"
	S3Plugin        = "s3"
	PostgresPlugin  = "postgres"
	LogPlugin       = "log"

	sourceConnector = "source"
	destConnector   = "destination"
)

// ValidatePlugin returns an error when the plugin name is not
// one of allowed Conduit built-in plugins
// or in one of the allowed GitHub organizations.
func ValidatePlugin(name string) error {
	trimmedName := strings.ToLower(strings.Trim(name, " "))

	if slices.Contains(
		BuiltinConnectors,
		strings.TrimPrefix(trimmedName, "builtin:"),
	) {
		return nil
	}

	for _, fc := range forbiddenConnectors {
		if trimmedName == fc {
			return fmt.Errorf("plugin %q is not supported", name)
		}
	}

	for _, org := range allowedGitHubOrgs {
		switch {
		case strings.HasPrefix(trimmedName, org):
			return nil
		case strings.HasPrefix(trimmedName, "github.com/"+org):
			return nil
		}
	}

	return fmt.Errorf("plugin %q is not supported", name)
}

func ValidatePluginType(t string) error {
	switch t {
	case sourceConnector, destConnector:
	default:
		return fmt.Errorf("plugin type %q is not supported", t)
	}

	return nil
}
