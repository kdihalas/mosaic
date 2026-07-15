package source_test

import (
	"bytes"
	"testing"

	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

func TestNewFilePreservesInput(t *testing.T) {
	content := []byte{'a', 0xff}
	file := source.NewFile("memory.mosaic", content)
	if file.Name != "memory.mosaic" || !bytes.Equal(file.Content, content) {
		t.Fatalf("NewFile() = %#v", file)
	}
}
