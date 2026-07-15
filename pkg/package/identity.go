// Package mosaicpackage defines Mosaic package manifests and identities.
package mosaicpackage

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var identitySegment = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Identity is the registry-independent name declared by a package manifest.
type Identity string

// ParseIdentity validates and returns a package identity.
func ParseIdentity(raw string) (Identity, error) {
	if raw == "" || strings.HasPrefix(raw, "/") || strings.HasSuffix(raw, "/") {
		return "", fmt.Errorf("identity must contain non-empty slash-separated segments")
	}
	if strings.Contains(raw, "..") || strings.ContainsAny(raw, `\\:@`) || strings.Contains(raw, "://") {
		return "", fmt.Errorf("identity contains a forbidden path, version, or URI component")
	}
	for _, segment := range strings.Split(raw, "/") {
		if !identitySegment.MatchString(segment) {
			return "", fmt.Errorf("invalid identity segment %q", segment)
		}
	}
	return Identity(raw), nil
}

func (i Identity) String() string { return string(i) }

// Version is an exact semantic package version.
type Version string

// ParseVersion accepts exact semantic versions only.
func ParseVersion(raw string) (Version, error) {
	if raw == "" || strings.HasPrefix(raw, "v") {
		return "", fmt.Errorf("version must be an exact semantic version without a v prefix")
	}
	v, err := semver.StrictNewVersion(raw)
	if err != nil {
		return "", err
	}
	if v.Original() != raw {
		return "", fmt.Errorf("version must be canonical semantic version syntax")
	}
	return Version(raw), nil
}

func (v Version) String() string { return string(v) }

// Semver returns the parsed semantic version. Validated Version values cannot fail.
func (v Version) Semver() *semver.Version {
	x, _ := semver.StrictNewVersion(string(v))
	return x
}

// SymbolID returns an alias-independent semantic identity.
func SymbolID(identity Identity, version Version, category, name string) string {
	return "package." + identity.String() + "@" + version.String() + "." + category + "." + name
}
