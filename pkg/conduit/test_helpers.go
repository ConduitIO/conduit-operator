package conduit

import (
	_ "embed"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/conduitio/conduit-operator/pkg/conduit/mock"
	"github.com/golang/mock/gomock"
)

//go:embed testdata/connector-example.yaml
var connectorYAMLResp string

//go:embed testdata/connector-list.json
var connectorListResp string

//go:embed testdata/proc.wasm
var procWASMResp string

func SetupHTTPMockClient(t *testing.T) *mock.MockhttpClient {
	t.Helper()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockhttpClient(ctrl)
	HTTPClient = mockClient

	return mockClient
}

func GetHTTPResps(t *testing.T) map[string]func(*http.Request) (*http.Response, error) {
	t.Helper()
	respMap := make(map[string]func(*http.Request) (*http.Response, error))

	respFn := func(_ *http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(connectorListResp)),
		}
		return resp, nil
	}
	respMap["list"] = respFn

	respFn = func(_ *http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(connectorYAMLResp)),
		}
		return resp, nil
	}
	respMap["spec"] = respFn

	respFn = func(_ *http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(procWASMResp)),
		}
		return resp, nil
	}
	respMap["wasm"] = respFn

	return respMap
}
