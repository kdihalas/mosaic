package packagearchive

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func contentDigest(files []File) string {
	h := sha256.New()
	var size [8]byte
	for _, file := range files {
		binary.BigEndian.PutUint64(size[:], uint64(len(file.Path)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(file.Path))
		binary.BigEndian.PutUint64(size[:], uint64(len(file.Data)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(file.Data)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func canonicalPath(name string) (string, bool) {
	if name == "" || !utf8.ValidString(name) || strings.Contains(name, `\`) || strings.ContainsRune(name, 0) {
		return "", false
	}
	if strings.HasPrefix(name, "/") || len(name) >= 2 && name[1] == ':' || path.IsAbs(name) {
		return "", false
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != name {
		return "", false
	}
	return clean, true
}

func sortFiles(files []File) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}

func caseKey(name string) string { return strings.ToLower(name) }
