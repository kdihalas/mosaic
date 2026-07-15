package lockfile

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

func Marshal(file File) ([]byte, error) {
	f := file.Sorted()
	if f.FormatVersion == "" {
		f.FormatVersion = FormatVersion
	}
	return toml.Marshal(f)
}

func Write(path string, file File) error {
	b, err := Marshal(file)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".mosaic-lock-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
