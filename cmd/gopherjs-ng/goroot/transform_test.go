package goroot

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kylelemons/godebug/diff"
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
// func SomeFunc() — GopherJS replacement at example.go:1:11

type SomeType struct{}
// func (SomeType) SomeMethod(b int) — GopherJS replacement at example.go:1:11

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

	keys := []string{}
	for k := range sf.WillPrune {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{
		"example.SomeAlias",
		"example.SomeConst",
		"example.SomeFunc",
		"example.SomeIface",
		"example.SomeType",
		"example.SomeType.SomeMethod",
		"example.SomeVar",
	}

	if diff := cmp.Diff(want, keys); diff != "" {
		t.Errorf("SymbolFilter.Collect() symbol keys differ from expected (-want,+got):\n%s", diff)
	}
}

func TestSymbolFilterPrune(t *testing.T) {
	filter := func(names ...string) SymbolFilter {
		fset := token.NewFileSet()
		f := fset.AddFile("example.go", fset.Base(), 42)
		sf := SymbolFilter{
			FileSet:   fset,
			WillPrune: map[string]token.Pos{},
		}
		for _, n := range names {
			sf.WillPrune[n] = f.Pos(10)
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
			filter:   filter("example.SomeFunc", "example.SomeType.SomeMethod"),
			original: exampleSource,
			want:     prunedSource,
		},
		{
			descr:    "func",
			filter:   filter("x.Func"),
			original: "package x; func Func() {}; func OtherFunc() {}",
			want:     "package x\n// func Func() — GopherJS replacement at example.go:1:11\nfunc OtherFunc() {}",
		},
		{
			descr:    "method",
			filter:   filter("x.T.M"),
			original: "package x; type T int; func (T) M() {}",
			want:     "package x; type T int\n // func (T) M() — GopherJS replacement at example.go:1:11",
		},
		{
			descr:    "single var",
			filter:   filter("x.V"),
			original: "package x; var V int = 1",
			want:     "package x\n// var V <abbreviated> — GopherJS replacement at example.go:1:11",
		},
		{
			descr:    "var group partial",
			filter:   filter("x.V1"),
			original: "package x; var (V1 int; V2 int)",
			want: "package x\n" +
				"// var V1 <abbreviated> — GopherJS replacement at example.go:1:11\n" +
				"var (V2 int)",
		},
		{
			descr:    "var group",
			filter:   filter("x.V1", "x.V2"),
			original: "package x; var (V1 int; V2 int)",
			want: "package x\n" +
				"// var V1 <abbreviated> — GopherJS replacement at example.go:1:11\n" +
				"// var V2 <abbreviated> — GopherJS replacement at example.go:1:11",
		},
		{
			descr:    "multi var partial",
			filter:   filter("x.V1"),
			original: "package x; func F() (int, int) {return 1, 2}; var V1, V2 = F()",
			want: "package x; func F() (int, int) {return 1, 2}\n" +
				"// var V1 <abbreviated> — GopherJS replacement at example.go:1:11\n" +
				"var _, V2 = F()\n",
		},
		{
			descr:    "multi var",
			filter:   filter("x.V1", "x.V2"),
			original: "package x; func F() (int, int) {return 1, 2}; var V1, V2 = F()",
			want: "package x; func F() (int, int) {return 1, 2}\n" +
				"// var V1 <abbreviated> — GopherJS replacement at example.go:1:11\n" +
				"// var V2 <abbreviated> — GopherJS replacement at example.go:1:11\n",
		},
		{
			descr:    "single const",
			filter:   filter("x.C"),
			original: "package x; const C int = 1",
			want:     "package x\n// const C <abbreviated> — GopherJS replacement at example.go:1:11",
		},
		{
			descr:    "const group",
			filter:   filter("x.C1"),
			original: "package x; const (C1 int = 1; C2 int = 2)",
			want: "package x\n" +
				"// const C1 <abbreviated> — GopherJS replacement at example.go:1:11\n" +
				"const (C2 int = 2)",
		},
		{
			descr:    "single type",
			filter:   filter("x.T1"),
			original: "package x; type T1 int; type T2 bool",
			want: "package x\n" +
				"// type T1 <abbreviated> — GopherJS replacement at example.go:1:11\n\n" +
				"type T2 bool",
		},
		{
			descr:    "multiline type",
			filter:   filter("x.T1"),
			original: "package x; type T1 struct {A int; B int; C string;}",
			want: "package x;\n" +
				"// type T1 <abbreviated> — GopherJS replacement at example.go:1:11",
		},
		{
			descr:    "type group",
			filter:   filter("x.T1", "x.T2"),
			original: "package x; type (T1 int; T2 bool)",
			want: "package x\n" +
				"// type T1 <abbreviated> — GopherJS replacement at example.go:1:11\n" +
				"// type T2 <abbreviated> — GopherJS replacement at example.go:1:11",
		},
	}

	for _, test := range tests {
		t.Run(test.descr, func(t *testing.T) {
			fset := test.filter.FileSet

			f := parse(t, fset, gofmt(t, test.original))
			test.filter.Prune(f)
			got := reconstruct(t, fset, f)

			if diff := diff.Diff(gofmt(t, test.want), got); diff != "" {
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

func reconstruct(t *testing.T, fset *token.FileSet, f *ast.File) string {
	t.Helper()
	buf := &strings.Builder{}
	if err := printer.Fprint(buf, fset, f); err != nil {
		t.Fatalf("Failed to format ast: %s", err)
	}
	return buf.String()
}

func gofmt(t *testing.T, src string) string {
	t.Helper()
	fset := token.NewFileSet()
	f := parse(t, fset, src)
	return reconstruct(t, fset, f)
}
