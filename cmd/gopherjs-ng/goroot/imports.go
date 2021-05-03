package goroot

import (
	"go/ast"
	"go/token"
	"strconv"
)

// nosync rewrites "sync" imports with our own "nosync". See nosyncPkgs comment
// for details.
func nosync(fset *token.FileSet, f *ast.File) bool {
	modified := false
	for _, spec := range f.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path == "sync" {
			if spec.Name == nil {
				spec.Name = ast.NewIdent("sync")
			}
			spec.Path.Value = `"github.com/gopherjs/gopherjs/nosync"`
			modified = true
		}
	}

	return modified
}
