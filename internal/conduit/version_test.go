package conduit_test

import (
	"testing"

	"github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/conduit"
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
				"--processors.path", "/conduit.storage/processors",
			},
		},
		{
			name:    "with version greater than 0.12",
			version: "v0.13.0",
			want: []string{
				"run",
				"--pipelines.path", "/conduit.pipelines/pipeline.yaml",
				"--connectors.path", "/conduit.storage/connectors",
				"--db.type", "sqlite",
				"--db.sqlite.path", "/conduit.storage/db",
				"--pipelines.exit-on-degraded",
				"--processors.path", "/conduit.storage/processors",
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

			flags := conduit.NewFlags(
				conduit.WithPipelineFile(v1alpha.ConduitPipelineFile),
				conduit.WithConnectorsPath(v1alpha.ConduitConnectorsPath),
				conduit.WithDBPath(v1alpha.ConduitDBPath),
				conduit.WithProcessorsPath(v1alpha.ConduitProcessorsPath),
			)
			args, err := flags.ForVersion(tc.version)
			if err != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			}
			is.Equal(args, tc.want)
		})
	}
}
