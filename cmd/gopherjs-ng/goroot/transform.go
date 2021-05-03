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
	"strings"

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
type SymbolFilter struct {
	// FileSet that was used to parse files the filter will be working with.
	FileSet *token.FileSet
	// Mapping of symbol names to positions where they were found.
	WillPrune map[string]token.Pos
}

func (sf *SymbolFilter) funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	recv := d.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	return recv.(*ast.Ident).Name + "." + d.Name.Name
}

// key generates a key for a named symbol that is used to detect, which original
// symbols are to be replaced with an augmentation. Keys are prefixed with file's
// package name in order to distinguish between somepackage and somepackage_test.
func (sf *SymbolFilter) key(f *ast.File, n ast.Node) string {
	switch n := n.(type) {
	case *ast.TypeSpec:
		return f.Name.Name + "." + n.Name.Name
	case *ast.FuncDecl:
		return f.Name.Name + "." + sf.funcName(n)
	case *ast.Ident: // For top-level variables and constants.
		return f.Name.Name + "." + n.Name
	default:
		panic(fmt.Errorf("AST node %v is not supported by SymbolFilter", n))
	}
}

// Collect names of top-level symbols in the source file. Doesn't modify the
// file itself and always returns false.
func (sf *SymbolFilter) Collect(f *ast.File) bool {
	if sf.WillPrune == nil {
		sf.WillPrune = map[string]token.Pos{}
	}
	collectName := func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.File: // Root node.
			return true
		case *ast.GenDecl: // Import, const, var or type declaration, child of *ast.File.
			return node.Tok != token.IMPORT
		case *ast.ValueSpec: // Const or var spec, child of *ast.GenDecl.
			for _, name := range node.Names {
				sf.WillPrune[sf.key(f, name)] = name.Pos()
			}
		case *ast.TypeSpec: // Type spec, child of *ast.GenDecl.
			sf.WillPrune[sf.key(f, node)] = node.Pos()
		case *ast.FuncDecl: // Function or method declaration, child of *ast.File.
			sf.WillPrune[sf.key(f, node)] = node.Pos()
		}
		return false // By default, don't traverse child nodes.
	}
	astutil.Apply(f, collectName, nil)
	return false
}

// Prune in-place top-level symbols with names that match previously collected.
//
// For each pruned symbol adds a comment naming the sympol and referencing a
// place where the replacement is. Returns true if any modifications were made.
func (sf *SymbolFilter) Prune(f *ast.File) bool {
	if sf.IsEmpty() {
		return false // Empty filter won't prune anything.
	}
	pruned := false
	visitNode := func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.File: // Root node.
			return true
		case *ast.GenDecl: // Import, const, var or type declaration, child of *ast.File.
			return node.Tok != token.IMPORT
		case *ast.FuncDecl: // Function or method declaration, child of *ast.File.
			if pos, ok := sf.WillPrune[sf.key(f, node)]; ok {
				f.Comments = append(f.Comments, sf.placeholder(&ast.FuncDecl{
					Name: node.Name,
					Recv: node.Recv,
					Type: node.Type,
				}, node.Pos(), pos))
				c.Delete()
				pruned = true
			}
		case *ast.ValueSpec: // Const or var spec, child of *ast.GenDecl.
			parent := c.Parent().(*ast.GenDecl)
			remaining := len(node.Names)
			// Var and const declarations may have multiple names, for example:
			// `var a, b = foo()`. Process them individually.
			for i, name := range node.Names {
				if pos, ok := sf.WillPrune[sf.key(f, name)]; ok {
					f.Comments = append(f.Comments, sf.placeholder(&ast.GenDecl{
						Tok: parent.Tok,
						Specs: []ast.Spec{&ast.ValueSpec{
							Names: []*ast.Ident{node.Names[i]},
							Type:  ast.NewIdent("<abbreviated>"),
						}},
						TokPos: parent.TokPos,
					}, c.Parent().Pos()-1, pos))

					// Deleting individual var/const names from a declaration is unsafe,
					// since they need to be kept in sync with initialization exprs.
					// In that case we simply rename the variable to '_', which the compiler
					// will ignore.
					node.Names[i] = ast.NewIdent("_")
					remaining--
					pruned = true
				}
			}
			// If all names were removed, we can delete the whole ValueSpec and avoid
			// initialization expression pinning potentially dead code.
			if remaining == 0 {
				c.Delete()
			}
		case *ast.TypeSpec: // Type spec, child of *ast.GenDecl.
			if pos, ok := sf.WillPrune[sf.key(f, node)]; ok {
				f.Comments = append(f.Comments, sf.placeholder(&ast.GenDecl{
					Tok: token.TYPE,
					Specs: []ast.Spec{&ast.TypeSpec{
						Name: node.Name,
						Type: ast.NewIdent("<abbreviated>"),
					}},
					TokPos: c.Parent().Pos(),
				}, c.Parent().Pos()-1, pos))
				c.Delete()
				pruned = true
			}
		}
		return false
	}

	pruneEmptyDecls := func(c *astutil.Cursor) bool {
		d, ok := c.Node().(*ast.GenDecl)
		if !ok {
			return true
		}
		if len(d.Specs) == 0 {
			// If all child const/var/type specs were deleted, remove the parent decl.
			c.Delete()
		}
		return true
	}
	astutil.Apply(f, visitNode, pruneEmptyDecls)
	return pruned
}

// IsEmpty returns true if no symbols are going to be pruned by this filter.
func (sf *SymbolFilter) IsEmpty() bool { return len(sf.WillPrune) == 0 }

var emptyFSet = token.NewFileSet()

// placeholder generates a comment for a pruned AST node with a pointer to where the replacement is.
func (sf *SymbolFilter) placeholder(n ast.Node, origPos, replPos token.Pos) *ast.CommentGroup {
	buf := &strings.Builder{}
	err := format.Node(buf, emptyFSet, n)
	if err != nil {
		// Should never happen.
		panic(fmt.Errorf("failed to format AST node %v: %w", n, err))
	}
	// Just in case printed source ends up multi-line despite of the trimming above,
	// make sure all lines are commented out.
	str := strings.ReplaceAll(buf.String(), "\n", "\n// ")

	return &ast.CommentGroup{
		List: []*ast.Comment{{
			Slash: origPos,
			Text:  fmt.Sprintf("// %s — GopherJS replacement at %s", str, sf.position(replPos)),
		}},
	}
}

func (sf *SymbolFilter) position(pos token.Pos) token.Position {
	if sf.FileSet == nil {
		return token.Position{}
	}
	return sf.FileSet.Position(pos)
}

type astTransformer func(*ast.File) bool

func (sf *SymbolFilter) processSource(loadFS http.FileSystem, loadPath, writePath string, processor astTransformer) error {
	source, err := loadAST(sf.FileSet, loadFS, loadPath, writePath)
	if err != nil {
		return fmt.Errorf("failed to load %q AST: %w", loadPath, err)
	}

	if !processor(source) {
		// Optimization: if no modifications were made, no need to rebuild source code
		// from AST.
		return copyUnmodified(loadFS, loadPath, writePath)
	}

	if err := writeAST(sf.FileSet, writePath, source); err != nil {
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
