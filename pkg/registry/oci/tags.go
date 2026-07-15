package oci

import (
	"context"
	"sort"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
)

func (c *Client) Tags(ctx context.Context, raw string) ([]string, diagnostics.List) {
	ref, err := ParseReference(raw)
	if err != nil {
		return nil, ociDiagnostic("OCI001", err.Error(), raw)
	}
	repo, err := c.repository(ref)
	if err != nil {
		return nil, ociDiagnostic("OCI001", err.Error(), raw)
	}
	var tags []string
	err = repo.Tags(ctx, "", func(page []string) error {
		tags = append(tags, page...)
		if len(tags) > c.options.Limits.MaxAvailableVersions {
			return errTooManyTags{}
		}
		return nil
	})
	if err != nil {
		return nil, ociError(err, raw)
	}
	sort.Strings(tags)
	return tags, nil
}

type errTooManyTags struct{}

func (errTooManyTags) Error() string { return "registry tag list exceeds configured limit" }
