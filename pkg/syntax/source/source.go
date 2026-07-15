// Package source defines explicit in-memory source files.
package source

// File is a named, exact sequence of source bytes. Lexers and later compiler
// phases must not mutate Content.
type File struct {
	Name    string
	Content []byte
}

// NewFile constructs an in-memory source file.
func NewFile(name string, content []byte) File {
	return File{Name: name, Content: content}
}
