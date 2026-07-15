// Package dependency resolves deterministic Mosaic package dependency graphs.
package dependency

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// Constraint is a validated semantic version constraint.
type Constraint struct {
	raw    string
	parsed *semver.Constraints
}

func ParseConstraint(raw string) (Constraint, error) {
	if raw == "" {
		return Constraint{}, fmt.Errorf("version constraint is empty")
	}
	c, err := semver.NewConstraint(raw)
	if err != nil {
		return Constraint{}, err
	}
	return Constraint{raw: raw, parsed: c}, nil
}

func (c Constraint) String() string { return c.raw }

func (c Constraint) Check(version *semver.Version, includePrerelease bool) bool {
	if c.parsed == nil || version == nil {
		return false
	}
	if c.parsed.Check(version) {
		return true
	}
	if includePrerelease && version.Prerelease() != "" {
		stable, _ := semver.NewVersion(fmt.Sprintf("%d.%d.%d", version.Major(), version.Minor(), version.Patch()))
		return c.parsed.Check(stable)
	}
	return false
}
