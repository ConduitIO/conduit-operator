package v1alpha

import (
	"context"
	"errors"
	"fmt"
	"testing"

	v1alpha "github.com/conduitio/conduit-operator/api/v1alpha"
	"github.com/conduitio/conduit-operator/internal/testutil"
	"github.com/conduitio/conduit-operator/pkg/conduit"
	"github.com/golang/mock/gomock"
	"github.com/matryer/is"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestWebhookValidate_ConduitVersion(t *testing.T) {
	tests := []struct {
		ver         string
		expectedErr error
	}{
		{ver: "v0.13.2", expectedErr: nil},
		{ver: "v1", expectedErr: nil},
		{ver: "v0.10.2", expectedErr: fmt.Errorf(`spec.version: Invalid value: "v0.10.2": unsupported conduit version "v0.10.2", minimum required "v0.11.0"`)},
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
			ctx := context.Background()
			cl := fake.NewClientBuilder().Build()
			v := &ConduitCustomValidator{conduit.NewValidator(ctx, cl, log.Log.WithName("webhook-validation"))}

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

func TestWebhook_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr error
	}{
		{
			name: "validation is successful",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
				)

				return testutil.SetupSampleConduit(t)
			},
		},
		{
			name: "validates pipeline with source processors",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
				)

				return testutil.SetupSourceProcConduit(t)
			},
		},
		{
			name: "error occurs for connector params",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
				)

				return testutil.SetupSampleConduit(t)
			},
			wantErr: nil,
		},
		{
			name: "error occurs for standalone processor schema",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")),
				)

				return testutil.SetupSampleConduit(t)
			},
			wantErr: apierrors.NewInvalid(v1alpha.GroupKind, "sample", field.ErrorList{
				field.InternalError(
					field.NewPath("spec", "processors", "standalone"),
					fmt.Errorf("failed to save wasm to file: BOOM"),
				),
			}),
		},
		{
			name: "error occurs during validation",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
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
			ctx := context.Background()
			c := tc.setup()
			cl := fake.NewClientBuilder().Build()
			v := ConduitCustomValidator{
				conduit.NewValidator(ctx, cl, log.Log.WithName("webhook-validation")),
			}

			_, err := v.ValidateCreate(context.Background(), runtime.Object(c))

			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				is.True(err == nil)
			}
		})
	}
}

func TestWebhook_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *v1alpha.Conduit
		wantErr error
	}{
		{
			name: "validation is successful",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpFnResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps["wasm"]),
				)
				// mock registry

				return testutil.SetupSampleConduit(t)
			},
		},
		{
			name: "validates pipeline with source processors",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
				)

				return testutil.SetupSourceProcConduit(t)
			},
		},
		{
			name: "error occurs on http call in connector schema validation",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
				)

				return testutil.SetupSampleConduit(t)
			},
			wantErr: nil,
		},
		{
			name: "error occurs during standalone proc registration",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["wasm"]),
				)

				return testutil.SetupSampleConduit(t)
			},
		},
		{
			name: "error occurs for connector processor validation",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).Return(nil, errors.New("BOOM")),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpResps["spec"]),
				)

				return testutil.SetupSourceProcConduit(t)
			},
			wantErr: apierrors.NewInvalid(v1alpha.GroupKind, "sample", field.ErrorList{
				field.InternalError(
					field.NewPath("spec", "connectors", "standalone"),
					fmt.Errorf("failed to save wasm to file: BOOM"),
				),
			}),
		},
		{
			name: "error occurs during connector param validation",
			setup: func() *v1alpha.Conduit {
				webClient := conduit.SetupHTTPMockClient(t)
				httpFnResps := conduit.GetHTTPResps(t)
				gomock.InOrder(
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps["list"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps["spec"]),
					webClient.EXPECT().Do(gomock.Any()).DoAndReturn(httpFnResps["spec"]),
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
			ctx := context.Background()
			c := tc.setup()
			cl := fake.NewClientBuilder().Build()
			v := ConduitCustomValidator{
				conduit.NewValidator(ctx, cl, log.Log.WithName("webhook-validation")),
			}

			_, err := v.ValidateUpdate(context.Background(), runtime.Object(nil), runtime.Object(c))

			if tc.wantErr != nil {
				is.True(err != nil)
				is.Equal(err.Error(), tc.wantErr.Error())
			} else {
				fmt.Printf("err %s\n", err)
				is.True(err == nil)
			}
		})
	}
}
