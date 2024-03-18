package controllers

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/conduitio-labs/conduit-operator/api/v1"
)

func init() {
	// validate constraint
	constraint := fmt.Sprint(">= ", v1.ConduitWithProcessorsVersion)
	if _, err := semver.NewConstraint(constraint); err != nil {
		panic(fmt.Errorf("failed to create version constraint: %w", err))
	}
}

// WithProcessors returns true when Conduit supports the new processors sdk.
// Returns false when Conduit does not offer support or when the version cannot be parsed.
func withProcessors(ver string) bool {
	sanitized, _ := strings.CutPrefix(ver, "v")
	v, err := semver.NewVersion(sanitized)
	if err != nil {
		return false
	}
	c, _ := semver.NewConstraint(fmt.Sprint(">= ", v1.ConduitWithProcessorsVersion))
	return c.Check(v)
}
