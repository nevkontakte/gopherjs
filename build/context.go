package build

import (
	"go/build"
	"net/http"
	"path"
)

// buildCtx is a common interface for a variety of different contexts
// GopherJS can get package sources.
//
// It is generally can be thought of as abstract and extended go/build.Context.
type buildCtx interface {
	Import(path string, srcDir string, mode build.ImportMode) (*PackageData, error)
}

// embeddedCtx is a build context for packages embedded into the GopherJS
// binary, such as natives and nosync/js packages.
type embeddedCtx struct {
	bctx build.Context
}

func newEmbeddedCtx(embedded http.FileSystem, GOOS, GOARCH string) *embeddedCtx {
	fs := &vfs{embedded}
	ec := embeddedCtx{
		bctx: build.Context{
			GOROOT:   "/",
			GOPATH:   "/",
			GOOS:     GOOS,
			GOARCH:   GOARCH,
			Compiler: "gc",

			// path functions must behave unix-like to work with the VFS.
			JoinPath:      path.Join,
			SplitPathList: splitPathList,
			IsAbsPath:     path.IsAbs,

			// Substitute real FS with the embedded one.
			IsDir:     fs.IsDir,
			HasSubdir: fs.HasSubDir,
			ReadDir:   fs.ReadDir,
			OpenFile:  fs.OpenFile,
		},
	}
	return &ec
}

// Import returns details about the Go package named by the importPath, interpreting local import paths relative to the srcDir directory.
func (ec embeddedCtx) Import(importPath string, srcDir string, mode build.ImportMode) (*PackageData, error) {
	pkg, err := ec.bctx.Import(importPath, srcDir, build.FindOnly)
	if err != nil {
		return nil, err
	}
	return &PackageData{
		Package:   pkg,
		IsVirtual: true,
		JSFiles:   nil, // .js.inc files are not supported on VFS.
	}, nil
}
