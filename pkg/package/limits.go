package mosaicpackage

// Limits bounds untrusted package and registry input.
type Limits struct {
	MaxArchiveBytes          int64
	MaxUncompressedBytes     int64
	MaxFiles                 int
	MaxFileBytes             int64
	MaxDependencyDepth       int
	MaxDependencies          int
	MaxAvailableVersions     int
	MaxRegistryResponseBytes int64
}

// DefaultLimits returns practical, non-zero safety limits.
func DefaultLimits() Limits {
	return Limits{
		MaxArchiveBytes: 512 << 20, MaxUncompressedBytes: 2 << 30, MaxFiles: 100000,
		MaxFileBytes: 128 << 20, MaxDependencyDepth: 50, MaxDependencies: 500,
		MaxAvailableVersions: 10000, MaxRegistryResponseBytes: 32 << 20,
	}
}

// WithDefaults fills zero-valued limit fields.
func (l Limits) WithDefaults() Limits {
	d := DefaultLimits()
	if l.MaxArchiveBytes == 0 {
		l.MaxArchiveBytes = d.MaxArchiveBytes
	}
	if l.MaxUncompressedBytes == 0 {
		l.MaxUncompressedBytes = d.MaxUncompressedBytes
	}
	if l.MaxFiles == 0 {
		l.MaxFiles = d.MaxFiles
	}
	if l.MaxFileBytes == 0 {
		l.MaxFileBytes = d.MaxFileBytes
	}
	if l.MaxDependencyDepth == 0 {
		l.MaxDependencyDepth = d.MaxDependencyDepth
	}
	if l.MaxDependencies == 0 {
		l.MaxDependencies = d.MaxDependencies
	}
	if l.MaxAvailableVersions == 0 {
		l.MaxAvailableVersions = d.MaxAvailableVersions
	}
	if l.MaxRegistryResponseBytes == 0 {
		l.MaxRegistryResponseBytes = d.MaxRegistryResponseBytes
	}
	return l
}
