package v1alpha

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/golang/mock/gomock"
	"github.com/matryer/is"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

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

func TestValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr error
	}{
		{
			name: "validation is successful",
			setup: func() *v1alpha.Conduit {
				webClient, httpResp := setupHTTPMock(t)
				webClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(_ *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(strings.NewReader(connectorYAML)),
					}, nil
				}).Times(2)
				t.Cleanup(func() { httpResp.Body.Close() })

				return setupSampleConduit(t, true)
			},
		},
		{
			name: "error occurs",
			setup: func() *v1alpha.Conduit {
				webClient, httpResp := setupHTTPMock(t)
				webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")).Times(2)
				t.Cleanup(func() { httpResp.Body.Close() })

				return setupSampleConduit(t, true)
			},
			wantErr: apierrors.NewInvalid(v1alpha.GroupKind, "sample", field.ErrorList{
				field.InternalError(
					field.NewPath("spec", "connectors", "parameter"),
					fmt.Errorf("failed getting plugin params from cache with error getting yaml from cache with error BOOM")),
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

func TestValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr error
	}{
		{
			name: "validation is successful",
			setup: func() *v1alpha.Conduit {
				webClient, httpResp := setupHTTPMock(t)
				httpClient = webClient
				webClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(_ *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(strings.NewReader(connectorYAML)),
					}, nil
				}).Times(2)
				t.Cleanup(func() { httpResp.Body.Close() })

				return setupSampleConduit(t, true)
			},
		},
		{
			name: "error occurs",
			setup: func() *v1alpha.Conduit {
				webClient, httpResp := setupHTTPMock(t)
				httpClient = webClient
				webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")).Times(2)
				t.Cleanup(func() { httpResp.Body.Close() })

				return setupSampleConduit(t, true)
			},
			wantErr: apierrors.NewInvalid(v1alpha.GroupKind, "sample", field.ErrorList{
				field.InternalError(
					field.NewPath("spec", "connectors", "parameter"),
					fmt.Errorf("failed getting plugin params from cache with error getting yaml from cache with error BOOM")),
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
