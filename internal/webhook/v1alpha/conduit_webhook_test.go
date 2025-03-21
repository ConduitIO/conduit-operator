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
	"github.com/conduitio/conduit-operator/internal/testutil"
	"github.com/conduitio/conduit-operator/pkg/validator"
	"github.com/conduitio/conduit-operator/pkg/validator/mock"
	"github.com/golang/mock/gomock"
	"github.com/matryer/is"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			v := &ConduitCustomValidator{validator.NewConduitValidator()}

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

				return testutil.SetupSampleConduit(t)
			},
		},
		{
			name: "error occurs on http call",
			setup: func() *v1alpha.Conduit {
				webClient := setupHTTPMockClient(t)
				webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")).Times(4)

				return testutil.SetupSampleConduit(t)
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
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
				)

				return testutil.SetupBadValidationConduit(t)
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
			v := ConduitCustomValidator{validator.NewConduitValidator()}
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

				return testutil.SetupSampleConduit(t)
			},
		},
		{
			name: "error occurs on http call",
			setup: func() *v1alpha.Conduit {
				webClient := setupHTTPMockClient(t)
				webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")).Times(4)

				return testutil.SetupSampleConduit(t)
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
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps[1]),
				)

				return testutil.SetupBadValidationConduit(t)
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
			v := ConduitCustomValidator{validator.NewConduitValidator()}
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

func setupHTTPMockClient(t *testing.T) *mock.MockhttpClient {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockhttpClient(ctrl)
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
