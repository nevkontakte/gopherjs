package goroot

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"
)

// augmenter implements top-level symbol pruning for augmented packages.
//
// GopherJS standard library augmentations are done at the top-level symbol
// level, which allows to only keep a minimal subset of the code forked.
// augmenter implements logic that gathers symbol names from the overlay
// sources and then prunes their counterparts from the upstream sources, thus
// prevending conflicting symbol definitions.
//
// A singe augmenter instance is meant to be used with a single Go package.
// Methods Collect and Prune are meant to be used as astTransformers with the
// processSource() function.
//
// Not safe for concurrent use.
type augmenter struct {
	// Mapping of symbol names to positions where they were found.
	Replacements map[string]token.Pos
}

// Collect names of top-level symbols in the source file. Doesn't modify the
// file itself and always returns false.
func (a *augmenter) Collect(fset *token.FileSet, f *ast.File) bool {
	if a.Replacements == nil {
		a.Replacements = map[string]token.Pos{}
	}
	collectName := func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.File: // Root node.
			return true
		case *ast.GenDecl: // Import, const, var or type declaration, child of *ast.File.
			return node.Tok != token.IMPORT
		case *ast.ValueSpec: // Const or var spec, child of *ast.GenDecl.
			for _, name := range node.Names {
				a.Replacements[a.key(f, name)] = name.Pos()
			}
		case *ast.TypeSpec: // Type spec, child of *ast.GenDecl.
			a.Replacements[a.key(f, node)] = node.Pos()
		case *ast.FuncDecl: // Function or method declaration, child of *ast.File.
			a.Replacements[a.key(f, node)] = node.Pos()
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
func (a *augmenter) Prune(fset *token.FileSet, f *ast.File) bool {
	if a.IsEmpty() {
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
			if pos, ok := a.Replacements[a.key(f, node)]; ok {
				comment := a.placeholder(fset, token.FUNC, a.funcName(node), node.Pos(), pos)
				f.Comments = append(f.Comments, comment)
				c.Delete()
				pruned = true
			}
		case *ast.ValueSpec: // Const or var spec, child of *ast.GenDecl.
			parent := c.Parent().(*ast.GenDecl)
			remaining := len(node.Names)
			// Var and const declarations may have multiple names, for example:
			// `var a, b = foo()`. Process them individually.
			for i, name := range node.Names {
				if pos, ok := a.Replacements[a.key(f, name)]; ok {
					comment := a.placeholder(fset, parent.Tok, name.Name, c.Parent().Pos()-1, pos)
					f.Comments = append(f.Comments, comment)

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
			if pos, ok := a.Replacements[a.key(f, node)]; ok {
				comment := a.placeholder(fset, token.TYPE, node.Name.Name, c.Parent().Pos()-1, pos)
				f.Comments = append(f.Comments, comment)
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
func (a *augmenter) IsEmpty() bool { return len(a.Replacements) == 0 }

func (a *augmenter) funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	recv := d.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	return recv.(*ast.Ident).Name + "." + d.Name.Name
}

// key returns a string a string identifier for a top-level symbol. Overlay
// symbols will be treated as replacements for original symbols if their keys
// are identical. All top-level symbols in a well-formed Go package will have
// distinct keys.
func (a *augmenter) key(f *ast.File, n ast.Node) string {
	switch n := n.(type) {
	case *ast.TypeSpec:
		return f.Name.Name + "." + n.Name.Name
	case *ast.FuncDecl:
		return f.Name.Name + "." + a.funcName(n)
	case *ast.Ident: // For top-level variables and constants.
		return f.Name.Name + "." + n.Name
	default:
		panic(fmt.Errorf("AST node %v is not supported by augmenter", n))
	}
}

// placeholder generates a comment for a pruned AST node with a pointer to where the replacement is.
func (a *augmenter) placeholder(fset *token.FileSet, tok token.Token, name string, origPos, replPos token.Pos) *ast.CommentGroup {
	str := fmt.Sprintf("// %s %s — GopherJS replacement at %s", tok, name, fset.Position(replPos))
	return &ast.CommentGroup{
		List: []*ast.Comment{{
			Slash: origPos,
			Text:  str,
		}},
	}
}
