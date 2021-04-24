package goroot

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const exampleSource = `package example
func SomeFunc() {}
type SomeType struct{}
func (SomeType) SomeMethod(b int) {}
var SomeVar int
const SomeConst = 0
type SomeIface interface {
	SomeMethod(a int)
}
type SomeAlias = SomeType
`

const prunedSource = `package example
type SomeType struct{}
var SomeVar int
const SomeConst = 0
type SomeIface interface {
	SomeMethod(a int)
}
type SomeAlias = SomeType
`

func TestSymbolFilterCollect(t *testing.T) {
	f := parse(t, token.NewFileSet(), exampleSource)

	sf := SymbolFilter{}
	sf.Collect(f)

	want := SymbolFilter{
		"SomeFunc":            true,
		"SomeType":            true,
		"SomeType.SomeMethod": true,
		"SomeVar":             true,
		"SomeConst":           true,
		"SomeIface":           true,
		"SomeAlias":           true,
	}

	if diff := cmp.Diff(want, sf); diff != "" {
		t.Errorf("SymbolFilter.Collect() returned diff (-want,+got):\n%s", diff)
	}
}

func TestSymbolFilterPrune(t *testing.T) {
	filter := func(names ...string) SymbolFilter {
		sf := SymbolFilter{}
		for _, n := range names {
			sf[n] = true
		}
		return sf
	}

	tests := []struct {
		descr    string
		filter   SymbolFilter
		original string
		want     string
	}{
		{
			descr:    "smoke",
			filter:   filter("SomeFunc", "SomeType.SomeMethod"),
			original: exampleSource,
			want:     prunedSource,
		},
		{
			descr:    "func",
			filter:   filter("Func"),
			original: "package x; func Func() {}; func OtherFunc() {}",
			want:     "package x; func OtherFunc() {}",
		},
		{
			descr:    "method",
			filter:   filter("T.M"),
			original: "package x; type T int; func (T) M() {}",
			want:     "package x; type T int",
		},
		{
			descr:    "single var",
			filter:   filter("V"),
			original: "package x; var V int = 1",
			want:     "package x; var _ int = 1",
		},
		{
			descr:    "var group",
			filter:   filter("V1", "V2"),
			original: "package x; var (V1 int; V2 int)",
			want:     "package x; var (_ int; _ int)",
		},
		{
			descr:    "single const",
			filter:   filter("C"),
			original: "package x; const C int = 1",
			want:     "package x; const _ int = 1",
		},
		{
			descr:    "const group",
			filter:   filter("C1", "C2"),
			original: "package x; const (C1 int = 1; C2 int = 2)",
			want:     "package x; const (_ int = 1; _ int = 2)",
		},
		{
			descr:    "single type",
			filter:   filter("T1"),
			original: "package x; type T1 int; type T2 bool",
			want:     "package x; type T2 bool",
		},
		{
			descr:    "const group",
			filter:   filter("T1", "T2"),
			original: "package x; type (T1 int; T2 bool)",
			want:     "package x;",
		},
	}

	for _, test := range tests {
		t.Run(test.descr, func(t *testing.T) {
			fset := token.NewFileSet()

			f := parse(t, fset, test.original)
			test.filter.Prune(f)
			got := format(t, fset, f)

			if diff := cmp.Diff(reformat(t, test.want), got); diff != "" {
				t.Errorf("SymbolFilter.Prune() returned diff (-want,+got):\n%s", diff)
			}
		})
	}
}

func parse(t *testing.T, fset *token.FileSet, src string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(fset, "example.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse example source: %s", err)
	}
	return f
}

func format(t *testing.T, fset *token.FileSet, f *ast.File) string {
	t.Helper()
	buf := &strings.Builder{}
	if err := printer.Fprint(buf, fset, f); err != nil {
		t.Fatalf("Failed to format ast: %s", err)
	}
	return buf.String()
}

func reformat(t *testing.T, src string) string {
	t.Helper()
	fset := token.NewFileSet()
	f := parse(t, fset, src)
	return format(t, fset, f)
}
