package conduit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/testutil"
	"github.com/go-logr/logr/testr"
	"github.com/golang/mock/gomock"
	"github.com/matryer/is"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidator_ConnectorPlugin(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr *field.Error
	}{
		{
			name: "connector plugin is valid",
			setup: func() *v1alpha.Conduit {
				return testutil.SetupSampleConduit(t)
			},
		},
		{
			name: "connector plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := testutil.SetupSampleConduit(t)
				c.Spec.Connectors[0].Plugin = ""
				return c
			},
			wantErr: field.Invalid(
				field.NewPath("spec", "connectors", "plugin"),
				"",
				"plugin \"\" is not supported"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			c := tc.setup()
			fp := field.NewPath("spec").Child("connectors")
			logger := testr.New(t)
			ctx := context.Background()
			v := NewValidator(ctx, logger)

			err := v.ValidateConnector(ctx, c.Spec.Connectors[0], fp)
			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

func TestValidator_ConnectorPluginType(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr *field.Error
	}{
		{
			name: "connector plugin is valid",
			setup: func() *v1alpha.Conduit {
				return testutil.SetupSampleConduit(t)
			},
		},
		{
			name: "connector plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := testutil.SetupSampleConduit(t)
				c.Spec.Connectors[0].Type = ""
				return c
			},
			wantErr: field.Invalid(
				field.NewPath("spec", "connectors", "type"),
				"",
				"plugin type \"\" is not supported"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			c := tc.setup()
			logger := testr.New(t)
			fp := field.NewPath("spec").Child("connectors")
			ctx := context.Background()
			v := NewValidator(ctx, logger)

			err := v.ValidateConnector(ctx, c.Spec.Connectors[0], fp)
			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

func TestValidator_ProcessorPlugin(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr *field.Error
	}{
		{
			name: "processor plugin is valid",
			setup: func() *v1alpha.Conduit {
				return testutil.SetupSampleConduit(t)
			},
		},
		{
			name: "processor plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := testutil.SetupSampleConduit(t)
				c.Spec.Processors[0].Plugin = ""
				return c
			},
			wantErr: field.Required(
				field.NewPath("spec", "processors", "plugin"),
				"plugin cannot be empty"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			c := tc.setup()
			fp := field.NewPath("spec").Child("processors")
			logger := testr.New(t)
			ctx := context.Background()
			v := NewValidator(ctx, logger)

			err := v.ValidateProcessorPlugin(c.Spec.Processors[0], fp)
			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

//nolint:bodyclose // Body is closed in the validator, bodyclose is not recognizing this.
func TestValidator_ConnectorParameters(t *testing.T) {
	is := is.New(t)

	tests := []struct {
		name    string
		setup   func() *v1alpha.ConduitConnector
		wantErr error
	}{
		{
			name: "source connector parameters are valid",
			setup: func() *v1alpha.ConduitConnector {
				conduit := testutil.SetupSampleConduit(t)

				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
				)

				return conduit.Spec.Connectors[0]
			},
		},
		{
			name: "destination connector parameters are valid",
			setup: func() *v1alpha.ConduitConnector {
				conduit := testutil.SetupSampleConduit(t)

				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
				)

				return conduit.Spec.Connectors[1]
			},
		},
		{
			name: "bad connector name",
			setup: func() *v1alpha.ConduitConnector {
				conduit := testutil.SetupBadNameConduit(t)

				webClient := SetupHTTPMockClient(t)
				respFn := func(_ *http.Request) (*http.Response, error) {
					resp := &http.Response{
						StatusCode: http.StatusNotFound,
						Body:       io.NopCloser(bytes.NewBufferString("{\"message\": \"Not Found\"}")),
					}
					return resp, nil
				}

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(respFn),
				)

				return conduit.Spec.Connectors[0]
			},
			wantErr: nil,
		},
		{
			name: "error getting cached yaml",
			setup: func() *v1alpha.ConduitConnector {
				conduit := testutil.SetupSampleConduit(t)

				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")),
				)

				return conduit.Spec.Connectors[0]
			},
			wantErr: nil,
		},
		{
			name: "error constructing connector list",
			setup: func() *v1alpha.ConduitConnector {
				conduit := testutil.SetupSampleConduit(t)

				webClient := SetupHTTPMockClient(t)

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")),
				)

				return conduit.Spec.Connectors[0]
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(_ *testing.T) {
			c := tc.setup()
			fp := field.NewPath("spec").Child("connectors")
			logger := testr.New(t)
			ctx := context.Background()
			v := NewValidator(ctx, logger)

			err := v.ValidateConnector(ctx, c, fp)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.Equal(err, nil)
			}
		})
	}
}
