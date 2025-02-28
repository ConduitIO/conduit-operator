package v1alpha

import (
	"testing"

	"github.com/matryer/is"
)

func TestWebhookValidate_ConduitVersion(t *testing.T) {
	tests := []struct {
		ver       string
		supported bool
	}{
		{ver: "v0.11.1", supported: true},
		{ver: "v0.12.0", supported: true},
		{ver: "v1", supported: true},
		{ver: "v0.10", supported: false},
		{ver: "v0.9.1", supported: false},
	}

	testname := func(b bool, ver string) string {
		if b {
			return "supported " + ver
		}
		return "unsupported " + ver
	}

	for _, tc := range tests {
		t.Run(testname(tc.supported, tc.ver), func(t *testing.T) {
			is := is.New(t)
			v := &ConduitCustomValidator{}
			is.Equal(v.validateConduitVersion(tc.ver), tc.supported)
		})
	}
}
