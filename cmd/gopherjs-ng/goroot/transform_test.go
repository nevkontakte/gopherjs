package goroot

import (
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
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "example.go", exampleSource, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse example source: %s", err)
	}

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
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "example.go", exampleSource, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse example source: %s", err)
	}

	sf := SymbolFilter{
		"SomeFunc":            true,
		"SomeType.SomeMethod": true,
	}
	sf.Prune(f)

	buf := &strings.Builder{}
	printer.Fprint(buf, fset, f)

	if diff := cmp.Diff(prunedSource, buf.String()); diff != "" {
		t.Errorf("SymbolFilter.Prune() returned diff (-want,+got):\n%s", diff)
	}
}
