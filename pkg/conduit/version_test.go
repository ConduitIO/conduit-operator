package conduit

import (
	"testing"

	"github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/matryer/is"
	"github.com/pkg/errors"
)

func Test_ForVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    []string
		wantErr error
	}{
		{
			name:    "with version less than 0.12",
			version: "v0.11.1",
			want: []string{
				"-pipelines.path", "/conduit.pipelines/pipeline.yaml",
				"-connectors.path", "/conduit.storage/connectors",
				"-db.type", "sqlite",
				"-db.sqlite.path", "/conduit.storage/db",
				"-pipelines.exit-on-error",
				"-processors.path", "/conduit.storage/processors",
				"-log.format", "json",
			},
		},
		{
			name:    "with version of 0.12",
			version: "v0.12.0",
			want: []string{
				"--pipelines.path", "/conduit.pipelines/pipeline.yaml",
				"--connectors.path", "/conduit.storage/connectors",
				"--db.type", "sqlite",
				"--db.sqlite.path", "/conduit.storage/db",
				"--pipelines.exit-on-degraded",
				"--pipelines.error-recovery.max-retries", "0",
				"--processors.path", "/conduit.storage/processors",
				"--log.format", "json",
			},
		},
		{
			name:    "with a patched version of 0.12",
			version: "v0.12.4",
			want: []string{
				"--pipelines.path", "/conduit.pipelines/pipeline.yaml",
				"--connectors.path", "/conduit.storage/connectors",
				"--db.type", "sqlite",
				"--db.sqlite.path", "/conduit.storage/db",
				"--pipelines.exit-on-degraded",
				"--pipelines.error-recovery.max-retries", "0",
				"--processors.path", "/conduit.storage/processors",
				"--log.format", "json",
			},
		},
		{
			name:    "with version greater than 0.13",
			version: "v0.13.0",
			want: []string{
				"run",
				"--pipelines.path", "/conduit.pipelines/pipeline.yaml",
				"--connectors.path", "/conduit.storage/connectors",
				"--db.type", "sqlite",
				"--db.sqlite.path", "/conduit.storage/db",
				"--pipelines.exit-on-degraded",
				"--pipelines.error-recovery.max-retries", "0",
				"--processors.path", "/conduit.storage/processors",
				"--log.format", "json",
			},
		},
		{
			name:    "with version greater than 0.13.4",
			version: "v0.13.4",
			want: []string{
				"run",
				"--pipelines.path", "/conduit.pipelines/pipeline.yaml",
				"--connectors.path", "/conduit.storage/connectors",
				"--db.type", "sqlite",
				"--db.sqlite.path", "/conduit.storage/db",
				"--pipelines.exit-on-degraded",
				"--pipelines.error-recovery.max-retries", "0",
				"--processors.path", "/conduit.storage/processors",
				"--log.format", "json",
			},
		},
		{
			name:    "with version greater than 0.13.5",
			version: "v0.13.5",
			want: []string{
				"run",
				"--pipelines.path", "/conduit.pipelines",
				"--connectors.path", "/conduit.storage/connectors",
				"--db.type", "sqlite",
				"--db.sqlite.path", "/conduit.storage/db",
				"--pipelines.exit-on-degraded",
				"--pipelines.error-recovery.max-retries", "0",
				"--processors.path", "/conduit.storage/processors",
				"--log.format", "json",
			},
		},
		{
			name:    "with an unsupported version",
			version: "v0.14.0",
			want:    nil,
			wantErr: errors.Errorf("version v0.14.0 not supported"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			flags := NewFlags(
				WithPipelineFile(v1alpha.ConduitPipelineFile),
				WithConnectorsPath(v1alpha.ConduitConnectorsPath),
				WithDBPath(v1alpha.ConduitDBPath),
				WithProcessorsPath(v1alpha.ConduitProcessorsPath),
				WithLogFormat(v1alpha.ConduitLogFormatJSON),
			)
			args, err := flags.ForVersion(tc.version)
			if err != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			}
			is.Equal(args, tc.want)
		})
	}
}
