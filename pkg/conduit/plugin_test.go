package conduit_test

import (
	"errors"
	"testing"

	"github.com/conduitio-labs/conduit-operator/pkg/conduit"
	"github.com/matryer/is"
)

func Test_ValidatePlugin(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		wantErr    error
	}{
		{
			name:       "with full url github organization",
			pluginName: "github.com/conduitio/foo-connector",
		},
		{
			name:       "with organization",
			pluginName: "conduitio/foo-connector",
		},
		{
			name:       "with capitalized organization",
			pluginName: "ConduitIO/foo-connector",
		},
		{
			name:       "with conduitio-labs organization",
			pluginName: "conduitio-labs/foo-connector",
		},
		{
			name:       "with meroxa organization",
			pluginName: "meroxa/foo-connector",
		},
		{
			name:       "with meroxa organization",
			pluginName: "meroxa/foo-connector",
		},
		{
			name:       "with builtin kafka",
			pluginName: "builtin:kafka",
		},
		{
			name:       "with builtin file",
			pluginName: "builtin:file",
			wantErr:    errors.New("plugin \"builtin:file\" is not supported"),
		},
		{
			name:       "not allowed: conduitio/conduit-connector-file",
			pluginName: "conduitio/conduit-connector-file",
			wantErr:    errors.New("plugin \"conduitio/conduit-connector-file\" is not supported"),
		},
		{
			name:       "not allowed: github.com/ConduitIO/conduit-connector-file",
			pluginName: "github.com/ConduitIO/conduit-connector-file",
			wantErr:    errors.New("plugin \"github.com/ConduitIO/conduit-connector-file\" is not supported"),
		},
		{
			name:       "with builtin generator",
			pluginName: "builtin:generator",
		},
		{
			name:       "with builtin log",
			pluginName: "builtin:log  ",
		},
		{
			name:       "with builtin s3",
			pluginName: "builtin:s3",
		},
		{
			name:       "with builtin s3",
			pluginName: "builtin:postgres",
		},
		{
			name:       "fails with invalid org",
			pluginName: "foobar/foo-connector",
			wantErr:    errors.New(`plugin "foobar/foo-connector" is not supported`),
		},
		{
			name:       "fails with invalid builtin",
			pluginName: "builtin:ranger",
			wantErr:    errors.New(`plugin "builtin:ranger" is not supported`),
		},
		{
			name:       "fails with empty",
			pluginName: "",
			wantErr:    errors.New(`plugin "" is not supported`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			err := conduit.ValidatePlugin(tc.pluginName)
			if tc.wantErr != nil {
				is.True(err != nil) // expected error
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}
		})
	}
}

func Test_ValidatePluginType(t *testing.T) {
	is := is.New(t)

	tests := []struct {
		name       string
		pluginType string
		wantErr    error
	}{
		{
			name:       "with source",
			pluginType: "source",
		},
		{
			name:       "with destination",
			pluginType: "destination",
		},
		{
			name:       "with invalid type",
			pluginType: "invalid",
			wantErr:    errors.New(`plugin type "invalid" is not supported`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			err := conduit.ValidatePluginType(tc.pluginType)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.NoErr(err)
			}
		})
	}
}
