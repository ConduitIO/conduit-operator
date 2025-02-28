package v1alpha

import (
	"fmt"
	"testing"

	"github.com/matryer/is"
)

func TestWebhookValidate_ConduitVersion(t *testing.T) {
	tests := []struct {
		ver         string
		expectedErr error
	}{
		{ver: "v0.11.1", expectedErr: nil},
		{ver: "v0.12.0", expectedErr: nil},
		{ver: "v1", expectedErr: nil},
		{ver: "v0.10", expectedErr: fmt.Errorf(`spec.version: Invalid value: "v0.10": unsupported conduit version "v0.10", minimum required "v0.11.1"`)},
		{ver: "v0.9.1", expectedErr: fmt.Errorf(`spec.version: Invalid value: "v0.9.1": unsupported conduit version "v0.9.1", minimum required "v0.11.1"`)},
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
