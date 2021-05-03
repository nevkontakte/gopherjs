package goroot

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const ioBufSize = 10 * 1024 // 10 KiB

// astTransformer processes file AST and makes any modifications in-place. If any
// modifications were made, it must return true.
type astTransformer func(fset *token.FileSet, f *ast.File) bool

// chain two transformers to be invoked one after another. The resulting
// transformer will return true if either of its components made any modifications.
func (left astTransformer) chain(right astTransformer) astTransformer {
	return func(fset *token.FileSet, f *ast.File) bool {
		modified := left(fset, f)
		modified = modified || right(fset, f)
		return modified
	}
}

// identity is a trivial astTransformer that does nothing.
func identity(*token.FileSet, *ast.File) bool { return false }

func processSource(fset *token.FileSet, loadFS http.FileSystem, loadPath, writePath string, transform astTransformer) error {
	source, err := loadAST(fset, loadFS, loadPath, writePath)
	if err != nil {
		return fmt.Errorf("failed to load %q AST: %w", loadPath, err)
	}

	if !transform(fset, source) {
		// Optimization: if no modifications were made, no need to rebuild source code
		// from AST.
		return copyUnmodified(loadFS, loadPath, writePath)
	}

	if err := writeAST(fset, writePath, source); err != nil {
		return fmt.Errorf("failed to write %q: %w", writePath, err)
	}
	return nil
}

func loadAST(fset *token.FileSet, fs http.FileSystem, loadPath, writePath string) (*ast.File, error) {
	f, err := fs.Open(loadPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parser.ParseFile(fset, filepath.Base(writePath), f, parser.ParseComments)
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

	return format.Node(bf, fset, source)
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
