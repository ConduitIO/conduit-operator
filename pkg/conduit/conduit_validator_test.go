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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			cl := fake.NewClientBuilder().Build()
			v := NewValidator(ctx, cl, logger)

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
			cl := fake.NewClientBuilder().Build()
			v := NewValidator(ctx, cl, logger)

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
			cl := fake.NewClientBuilder().Build()
			v := NewValidator(ctx, cl, logger)

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
		client  func() client.Client
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
			client: func() client.Client {
				return fake.NewClientBuilder().Build()
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
			client: func() client.Client {
				return fake.NewClientBuilder().Build()
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
			client: func() client.Client {
				return fake.NewClientBuilder().Build()
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
			client: func() client.Client {
				return fake.NewClientBuilder().Build()
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
			client: func() client.Client {
				return fake.NewClientBuilder().Build()
			},
			wantErr: nil,
		},
		{
			name: "successfully decodes secret",
			setup: func() *v1alpha.ConduitConnector {
				conduit := testutil.SetupSecretConduit(t)

				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
				)

				return conduit.Spec.Connectors[0]
			},
			client: func() client.Client {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "objref1",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"key1": []byte("secret-value"),
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithObjects(secret).
					Build()

				var clientObj client.Client = fakeClient

				return clientObj
			},
		},
		{
			name: "secret does not exist",
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
			client: func() client.Client {
				return fake.NewClientBuilder().Build()
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(_ *testing.T) {
			c := tc.setup()
			cl := tc.client()
			fp := field.NewPath("spec").Child("connectors")
			logger := testr.New(t)
			ctx := context.Background()
			v := NewValidator(ctx, cl, logger)

			err := v.ValidateConnector(ctx, c, fp)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.Equal(err, nil)
			}
		})
	}
}
