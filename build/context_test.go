package build

import (
	"fmt"
	"go/build"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gopherjs/gopherjs/compiler/gopherjspkg"
)

func TestEmbeddedCtx(t *testing.T) {
	fs := &withPrefix{gopherjspkg.FS, "/src/github.com/gopherjs/gopherjs/"}
	ec := newEmbeddedCtx(fs, build.Default.GOOS, "js")

	t.Run("exists", func(t *testing.T) {
		importPath := "github.com/gopherjs/gopherjs/js"
		got, err := ec.Import(importPath, "", 0)
		if err != nil {
			t.Fatalf("ec.Import(%q) returned error: %s. Want: no error.", importPath, err)
		}
		targetRoot := fmt.Sprintf("/pkg/%s_%s", ec.bctx.GOOS, ec.bctx.GOARCH)
		want := &PackageData{
			Package: &build.Package{
				Dir:           "/src/github.com/gopherjs/gopherjs/js",
				ImportPath:    "github.com/gopherjs/gopherjs/js",
				Root:          "/",
				SrcRoot:       "/src",
				PkgRoot:       "/pkg",
				PkgTargetRoot: targetRoot,
				BinDir:        "/bin",
				Goroot:        true,
				PkgObj:        targetRoot + "/github.com/gopherjs/gopherjs/js.a",
			},
			IsVirtual: true,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ec.Import(%q) returned diff (-want,+got):\n%s", importPath, diff)
		}
	})

	t.Run("not found", func(t *testing.T) {
		importPath := "pkg/not/exists"
		_, err := ec.Import(importPath, "", build.FindOnly)
		want := "cannot find package"
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ec.Import(%q) returned error: %s. Want error containing %q.", importPath, err, want)
		}
	})
}
