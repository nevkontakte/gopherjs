package goroot

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/ast/astutil"
)

const ioBufSize = 10 * 1024 // 10 KiB

// SymbolFilter implements top-level symbol pruning for augmented packages.
//
// GopherJS standard library augmentations are done at the top-level symbol
// level, which allows to only keep a minimal subset of the code forked.
// SymbolFilter implements logic that gathers symbol names from the overlay
// sources and then prunes their counterparts from the upstream sources, thus
// prevending conflicting symbol definitions.
type SymbolFilter map[string]bool

func (sf SymbolFilter) funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	recv := d.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	return recv.(*ast.Ident).Name + "." + d.Name.Name
}

// traverse top-level symbols within the file and prune top-level symbols for which keep() returned
// false.
//
// This function is functionally very similar to ast.FilterFile with two differences: it doesn't
// descend into interface methods and struct fields, and it preserves imports.
func (sf SymbolFilter) traverse(f *ast.File, keep func(name string) bool) bool {
	pruned := false
	astutil.Apply(f, func(c *astutil.Cursor) bool {
		switch d := c.Node().(type) {
		case *ast.File: // Root node.
			return true
		case *ast.FuncDecl: // Child of *ast.File.
			if !keep(sf.funcName(d)) {
				c.Delete()
				pruned = true
			}
		case *ast.GenDecl: // Child of *ast.File.
			return c.Name() == "Decls"
		case *ast.ValueSpec: // Child of *ast.GenDecl.
			for i, name := range d.Names {
				if !keep(name.Name) {
					// Deleting variable/const declarations is somewhat fiddly (need to keep many different
					// slices inside of *ast.ValueSpec in sync), so we simply rename it to "_", so that the
					// compiler will dimply ignore it.
					d.Names[i] = ast.NewIdent("_")
					pruned = true
				}
			}
		case *ast.TypeSpec: // Child of *ast.GenDecl.
			if !keep(d.Name.Name) {
				c.Delete()
				pruned = true
			}
		}
		return false
	}, nil)
	return pruned
}

// Collect names of top-level symbols in the source file. Doesn't modify the file itself and always returns false.
func (sf SymbolFilter) Collect(f *ast.File) bool {
	return sf.traverse(f, func(name string) bool {
		sf[name] = true
		return true
	})
}

// Prune in-place top-level symbols with names that match previously collected. Returns true if any modifications were made.
func (sf SymbolFilter) Prune(f *ast.File) bool {
	if sf.IsEmpty() {
		return false // Empty filter won't prune anything.
	}
	return sf.traverse(f, func(name string) bool {
		return !sf[name]
	})
}

// IsEmpty returns true if no symbols are going to be pruned by this filter.
func (sf SymbolFilter) IsEmpty() bool { return len(sf) == 0 }

type astTransformer func(*ast.File) bool

func processSource(loadFS http.FileSystem, loadPath, writePath string, processor astTransformer) error {
	fset := token.NewFileSet()
	source, err := loadAST(fset, loadFS, loadPath)
	if err != nil {
		return fmt.Errorf("failed to load %q AST: %w", loadPath, err)
	}

	if !processor(source) {
		// Optimization: if no modifications were made, no need to rebuild source code
		// from AST.
		return copyUnmodified(loadFS, loadPath, writePath)
	}

	if err := writeAST(fset, writePath, source); err != nil {
		return fmt.Errorf("failed to write %q: %w", writePath, err)
	}
	return nil
}

func loadAST(fset *token.FileSet, fs http.FileSystem, path string) (*ast.File, error) {
	f, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parser.ParseFile(fset, filepath.Base(path), f, parser.ParseComments)
}

func writeAST(fset *token.FileSet, path string, source *ast.File) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("file %q already exists", path)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Using buffered IO significantly improves performance here.
	bf := bufio.NewWriterSize(f, ioBufSize)
	defer bf.Flush()

	return printer.Fprint(bf, fset, source)
}

func copyUnmodified(loadFS http.FileSystem, loadPath, writePath string) error {
	if realFS, ok := loadFS.(http.Dir); ok {
		// Further optimization: if we are copying from the real file system, do
		// a symlink instead.
		return os.Symlink(filepath.Join(string(realFS), loadPath), writePath)
	}
	from, err := loadFS.Open(loadPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer from.Close()

	to, err := os.Create(writePath)
	if err != nil {
		return fmt.Errorf("failed to open destination file: %w", err)
	}
	defer to.Close()

	if _, err := io.Copy(to, from); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}
