package v1alpha

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/webhook/v1alpha/mock"
	"github.com/golang/mock/gomock"
	"github.com/matryer/is"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

//go:embed testdata/connector-example.yaml
var connectorYAML string

func TestValidator_ConnectorPlugin(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr *field.Error
	}{
		{
			name: "connector plugin is valid",
			setup: func() *v1alpha.Conduit {
				return setupSampleConduit(t, true)
			},
		},
		{
			name: "connector plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := setupSampleConduit(t, true)
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

			err := validateConnectorPlugin(c.Spec.Connectors[0], fp)
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
				return setupSampleConduit(t, true)
			},
		},
		{
			name: "connector plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := setupSampleConduit(t, true)
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
			fp := field.NewPath("spec").Child("connectors")

			err := validateConnectorPluginType(c.Spec.Connectors[0], fp)
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
				return setupSampleConduit(t, true)
			},
		},
		{
			name: "processor plugin is invalid",
			setup: func() *v1alpha.Conduit {
				c := setupSampleConduit(t, true)
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

			err := validateProcessorPlugin(c.Spec.Processors[0], fp)
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
				conduit := setupSampleConduit(t, true)

				webClient := setupHTTPMockClient(t)
				httpResps := getHTTPResps()

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
				)

				return conduit.Spec.Connectors[0]
			},
		},
		{
			name: "destination connector parameters are valid",
			setup: func() *v1alpha.ConduitConnector {
				conduit := setupSampleConduit(t, true)

				webClient := setupHTTPMockClient(t)
				httpResps := getHTTPResps()

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
				)

				return conduit.Spec.Connectors[1]
			},
		},
		{
			name: "bad connector name",
			setup: func() *v1alpha.ConduitConnector {
				conduit := setupBadNameConduit(t)

				webClient := setupHTTPMockClient(t)
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
				conduit := setupSampleConduit(t, true)

				webClient := setupHTTPMockClient(t)
				httpResps := getHTTPResps()

				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
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

			err := validateConnectorParameters(c, fp)
			if tc.wantErr != nil {
				is.Equal(tc.wantErr.Error(), err.Error())
			} else {
				is.Equal(err, nil)
			}
		})
	}
}

func setupSampleConduit(t *testing.T, running bool) *v1alpha.Conduit {
	t.Helper()

	is := is.New(t)
	defaulter := ConduitCustomDefaulter{}

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			Description: "my-description",
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:   "source-connector",
					Type:   "source",
					Plugin: "builtin:generator",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topics",
							Value: "input-topic",
						},
					},
				},
				{
					Name:   "destination-connector",
					Type:   "destination",
					Plugin: "builtin:log",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topic",
							Value: "output-topic",
						},
					},
				},
			},
			Processors: []*v1alpha.ConduitProcessor{
				{
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
				},
			},
		},
	}

	// apply defaults
	is.NoErr(defaulter.Default(context.Background(), c))

	return c
}

func setupBadNameConduit(t *testing.T) *v1alpha.Conduit {
	t.Helper()

	is := is.New(t)
	defaulter := ConduitCustomDefaulter{}
	running := true

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			Description: "my-description",
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:   "source-connector",
					Type:   "source",
					Plugin: "generator",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topics",
							Value: "input-topic",
						},
					},
				},
				{
					Name:   "destination-connector",
					Type:   "destination",
					Plugin: "builtin:log",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topic",
							Value: "output-topic",
						},
					},
				},
			},
		},
	}

	// apply defaults
	is.NoErr(defaulter.Default(context.Background(), c))

	return c
}

func setupBadValidationConduit(t *testing.T) *v1alpha.Conduit {
	t.Helper()

	is := is.New(t)
	defaulter := ConduitCustomDefaulter{}
	running := true

	c := &v1alpha.Conduit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample",
			Namespace: "sample",
		},
		Spec: v1alpha.ConduitSpec{
			Running:     &running,
			Name:        "my-pipeline",
			Description: "my-description",
			Connectors: []*v1alpha.ConduitConnector{
				{
					Name:   "source-connector",
					Type:   "source",
					Plugin: "builtin:kafka",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
					},
				},
				{
					Name:   "destination-connector",
					Type:   "destination",
					Plugin: "builtin:kafka",
					Settings: []v1alpha.SettingsVar{
						{
							Name:  "servers",
							Value: "127.0.0.1",
						},
						{
							Name:  "topic",
							Value: "output-topic",
						},
					},
				},
			},
		},
	}

	// apply defaults
	is.NoErr(defaulter.Default(context.Background(), c))

	return c
}

func setupHTTPMockClient(t *testing.T) *mock.MockHTTPClient {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockHTTPClient(ctrl)
	httpClient = mockClient

	return mockClient
}

//nolint:bodyclose // Body is closed in the validator, bodyclose is not recognizing this.
func getHTTPResps() []func(*http.Request) (*http.Response, error) {
	var resps []func(*http.Request) (*http.Response, error)

	respFn := func(_ *http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString("{\"tag_name\": \"v0.10.1\"}")),
		}
		return resp, nil
	}
	resps = append(resps, respFn)

	respFn = func(_ *http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(connectorYAML)),
		}
		return resp, nil
	}
	resps = append(resps, respFn)

	return resps
}
