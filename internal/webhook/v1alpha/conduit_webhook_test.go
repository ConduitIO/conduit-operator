package v1alpha

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/pkg/validator"
	"github.com/conduitio/conduit-operator/pkg/validator/mock"
	"github.com/golang/mock/gomock"
	"github.com/matryer/is"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

//go:embed testdata/connector-example.yaml
var connectorYAML string

func TestWebhookValidate_ConduitVersion(t *testing.T) {
	tests := []struct {
		ver         string
		expectedErr error
	}{
		{ver: "v0.13.2", expectedErr: nil},
		{ver: "v1", expectedErr: nil},
		{ver: "v0.11.1", expectedErr: fmt.Errorf(`spec.version: Invalid value: "v0.11.1": unsupported conduit version "v0.11.1", minimum required "v0.13.2"`)},
		{ver: "v0.12", expectedErr: fmt.Errorf(`spec.version: Invalid value: "v0.12": unsupported conduit version "v0.12", minimum required "v0.13.2"`)},
	}

	testname := func(err error, ver string) string {
		if err == nil {
			return "supported " + ver
		}
		return "unsupported " + ver
	}

	for _, tc := range tests {
		t.Run(testname(tc.expectedErr, tc.ver), func(t *testing.T) {
			is := is.New(t)
			v := &ConduitCustomValidator{}

			fieldErr := v.validateConduitVersion(tc.ver)
			if tc.expectedErr != nil {
				is.True(fieldErr != nil)
				is.Equal(fieldErr.Error(), tc.expectedErr.Error())
			} else {
				is.True(fieldErr == nil)
			}
		})
	}
}

//nolint:bodyclose
func TestWebhook_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr error
	}{
		{
			name: "validation is successful",
			setup: func() *v1alpha.Conduit {
				webClient := setupHTTPMockClient(t)
				httpResps := getHTTPResps()
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
				)

				return setupSampleConduit(t)
			},
		},
		{
			name: "error occurs on http call",
			setup: func() *v1alpha.Conduit {
				webClient := setupHTTPMockClient(t)
				webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")).Times(4)

				return setupSampleConduit(t)
			},
			wantErr: nil,
		},
		{
			name: "error occurs during validation",
			setup: func() *v1alpha.Conduit {
				webClient := setupHTTPMockClient(t)
				httpResps := getHTTPResps()
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
				)

				return setupBadValidationConduit(t)
			},
			wantErr: apierrors.NewInvalid(v1alpha.GroupKind, "sample", field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "connectors", "parameter"),
					"source",
					"error validating \"topics\": required parameter is not provided",
				),
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			v := ConduitCustomValidator{}
			conduit := tc.setup()

			_, err := v.ValidateCreate(context.Background(), runtime.Object(conduit))

			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

//nolint:bodyclose
func TestWebhook_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr error
	}{
		{
			name: "validation is successful",
			setup: func() *v1alpha.Conduit {
				webClient := setupHTTPMockClient(t)
				httpFnResps := getHTTPResps()
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps[1]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps[1]),
				)

				return setupSampleConduit(t)
			},
		},
		{
			name: "error occurs on http call",
			setup: func() *v1alpha.Conduit {
				webClient := setupHTTPMockClient(t)
				webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")).Times(4)

				return setupSampleConduit(t)
			},
			wantErr: nil,
		},
		{
			name: "error occurs during validation",
			setup: func() *v1alpha.Conduit {
				webClient := setupHTTPMockClient(t)
				httpResps := getHTTPResps()
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[0]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
				)

				return setupBadValidationConduit(t)
			},
			wantErr: apierrors.NewInvalid(v1alpha.GroupKind, "sample", field.ErrorList{
				field.Invalid(
					field.NewPath("spec", "connectors", "parameter"),
					"source",
					"error validating \"topics\": required parameter is not provided",
				),
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			v := ConduitCustomValidator{}
			conduit := tc.setup()

			_, err := v.ValidateUpdate(context.Background(), runtime.Object(nil), runtime.Object(conduit))

			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

func setupSampleConduit(t *testing.T) *v1alpha.Conduit {
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
	validator.HTTPClient = mockClient

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
