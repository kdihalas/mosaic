package lexer_test

import (
	"bytes"
	"testing"

	"github.com/kdihalas/mosaic/pkg/syntax/lexer"
	"github.com/kdihalas/mosaic/pkg/syntax/source"
)

func BenchmarkLexSmallModule(b *testing.B) {
	benchmarkLex(b, []byte(`module API(input: APIInput) { workload "api" { replicas = 3 } }`))
}

func BenchmarkLexLargeGeneratedProject(b *testing.B) {
	var content bytes.Buffer
	for i := 0; i < 2_000; i++ {
		content.WriteString("resource Workload item { replicas = 3 image = \"example/api:1\" }\n")
	}
	benchmarkLex(b, content.Bytes())
}

func BenchmarkLexCommentHeavySource(b *testing.B) {
	var content bytes.Buffer
	for i := 0; i < 2_000; i++ {
		content.WriteString("/* outer /* nested */ comment */ // trailing\n")
	}
	benchmarkLex(b, content.Bytes())
}

func benchmarkLex(b *testing.B, content []byte) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(content)))
	src := source.NewFile("benchmark.mosaic", content)
	for i := 0; i < b.N; i++ {
		lexer.Lex(src, lexer.Options{})
	}
}
