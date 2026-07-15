package lockfile

import (
	"bytes"
	"os"

	"github.com/kdihalas/mosaic/pkg/diagnostics"
	"github.com/pelletier/go-toml/v2"
)

func Read(path string) (*File, diagnostics.List) {
	b, err := os.ReadFile(path)
	if err != nil {
		code := "LOCK001"
		if !os.IsNotExist(err) {
			code = "LOCK005"
		}
		return nil, diagnostic(code, err.Error(), path)
	}
	return Parse(b, path)
}

func Parse(data []byte, source string) (*File, diagnostics.List) {
	var f File
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, diagnostic("LOCK005", err.Error(), source)
	}
	if ds := ValidateStructure(f, source); ds.HasErrors() {
		return &f, ds
	}
	return &f, nil
}

func diagnostic(code, message, source string) diagnostics.List {
	return diagnostics.List{{Code: code, Severity: diagnostics.SeverityError, Message: message, Span: diagnostics.Span{SourceName: source}, Suggestion: "run `mosaic deps resolve`"}}
}
