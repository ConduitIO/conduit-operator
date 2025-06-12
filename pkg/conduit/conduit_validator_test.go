package conduit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/testutil"
	"github.com/conduitio/conduit-operator/pkg/conduit/mock"
	"github.com/conduitio/conduit/pkg/plugin"
	"github.com/go-logr/logr/testr"
	"github.com/golang/mock/gomock"
	"github.com/matryer/is"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var standaloneProc = v1alpha.ConduitProcessor{
	ID:           "proc1",
	Name:         "proc1",
	Plugin:       "builtin:base64.encode",
	Workers:      2,
	Condition:    "{{ eq .Metadata.key \"pipeline\" }}",
	ProcessorURL: "http://127.0.0.1:8090/api/files/processors/RECORD_ID/FILENAME",
	Settings: []v1alpha.SettingsVar{
		{
			Name: "setting01",
			SecretRef: &corev1.SecretKeySelector{
				Key: "setting01-%p-key",
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "setting01-secret-name",
				},
			},
		},
		{
			Name:  "setting02",
			Value: "setting02-val",
		},
	},
}

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

			err := v.validateProcessorPlugin(c.Spec.Processors[0], fp)
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
				secretUsername := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "objref1",
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"key1": []byte("secret-username"),
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithObjects(secretUsername).
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
		{
			name: "secret lookup fails",
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
						Name: "objref0",
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
			wantErr: field.InternalError(
				field.NewPath("spec", "connectors", "parameter"),
				fmt.Errorf("getting secrets for connector source-connector: %s ", "{ setting: saslUsername, err: failed to get \"objref1\" secret: secrets \"objref1\" not found}"),
			),
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
				assert.NotNil(t, err)
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.Equal(err, nil)
			}
		})
	}
}

//nolint:bodyclose // Body is closed in the validator, bodyclose is not recognizing this.
func TestValidator_StandaloneProcessor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name    string
		setup   func() (*v1alpha.ConduitProcessor, PluginRegistry)
		wantErr error
	}{
		{
			name: "registration succeeds",
			setup: func() (*v1alpha.ConduitProcessor, PluginRegistry) {
				p := standaloneProc

				// create registry
				mockRegistry := mock.NewMockPluginRegistry(ctrl)
				name := plugin.NewFullName(plugin.PluginTypeStandalone, p.Name, "latest")
				mockRegistry.EXPECT().
					Register(gomock.Any(), gomock.Any()).
					Return(name, nil).
					Times(1)

				// mock call to get wasm
				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
				)

				return &p, mockRegistry
			},
		},
		{
			name: "builtin processor",
			setup: func() (*v1alpha.ConduitProcessor, PluginRegistry) {
				p := v1alpha.ConduitProcessor{
					ID:        "proc1",
					Name:      "proc1",
					Plugin:    "builtin:base64.encode",
					Workers:   2,
					Condition: "{{ eq .Metadata.key \"pipeline\" }}",
					Settings: []v1alpha.SettingsVar{
						{
							Name: "setting01",
							SecretRef: &corev1.SecretKeySelector{
								Key: "setting01-%p-key",
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "setting01-secret-name",
								},
							},
						},
						{
							Name:  "setting02",
							Value: "setting02-val",
						},
					},
				}

				// create registry
				mockRegistry := mock.NewMockPluginRegistry(ctrl)
				name := plugin.NewFullName(plugin.PluginTypeStandalone, p.Name, "latest")
				mockRegistry.EXPECT().
					Register(gomock.Any(), gomock.Any()).
					Return(name, nil).
					Times(0)

				// mock call to get wasm
				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
				)

				return &p, mockRegistry
			},
		},
		{
			name: "error during wasm check",
			setup: func() (*v1alpha.ConduitProcessor, PluginRegistry) {
				p := standaloneProc
				mockRegistry := mock.NewMockPluginRegistry(ctrl)

				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")),
				)

				return &p, mockRegistry
			},
			wantErr: field.InternalError(
				field.NewPath("spec", "processors", "standalone"),
				fmt.Errorf("failed to save wasm to file: BOOM"),
			),
		},
		{
			name: "error during registration",
			setup: func() (*v1alpha.ConduitProcessor, PluginRegistry) {
				p := standaloneProc

				// create registry
				mockRegistry := mock.NewMockPluginRegistry(ctrl)
				name := plugin.NewFullName(plugin.PluginTypeStandalone, p.Name, "latest")
				mockRegistry.EXPECT().
					Register(gomock.Any(), gomock.Any()).
					Return(name, errors.New("BOOM")).
					Times(1)

				// mock call to fail
				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
				)

				return &p, mockRegistry
			},
			wantErr: field.InternalError(
				field.NewPath("spec", "processors", "standalone"),
				fmt.Errorf("failed to register: BOOM"),
			),
		},
		{
			name: "plugin already registered",
			setup: func() (*v1alpha.ConduitProcessor, PluginRegistry) {
				p := standaloneProc

				// create registry
				mockRegistry := mock.NewMockPluginRegistry(ctrl)
				name := plugin.NewFullName(plugin.PluginTypeStandalone, p.Name, "latest")
				mockRegistry.EXPECT().
					Register(gomock.Any(), gomock.Any()).
					Return(name, plugin.ErrPluginAlreadyRegistered).
					Times(1)

				// mock call to fail
				webClient := SetupHTTPMockClient(t)
				httpResps := GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
				)

				return &p, mockRegistry
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(_ *testing.T) {
			is := is.New(t)
			logger := testr.New(t)
			ctx := context.Background()
			p, reg := tc.setup()
			cl := fake.NewClientBuilder().Build()
			v := NewValidator(ctx, cl, logger)
			fp := field.NewPath("spec").Child("processors")

			err := v.validateStandaloneProcessor(ctx, p, reg, fp)
			if tc.wantErr != nil {
				assert.NotNil(t, err)
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}
