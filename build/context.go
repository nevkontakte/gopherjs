package build

import (
	"fmt"
	"go/build"
	"net/http"
	"path"
	"strings"

	"github.com/gopherjs/gopherjs/compiler"
)

// buildCtx is a common interface for a variety of different contexts
// GopherJS can get package sources.
//
// It is generally can be thought of as abstract and extended go/build.Context.
type buildCtx interface {
	// Import returns details about the Go package named by the importPath,
	// interpreting local import paths relative to the srcDir directory.
	Import(path string, srcDir string, mode build.ImportMode) (*PackageData, error)
}

// simpleCtx adds GopherJS-specific metadata to packages imported by
// the underlying go/build.Context.
type simpleCtx struct {
	bctx      build.Context
	isVirtual bool // Imported packages don't have a physical directory on disk.
}

// Import implements buildCtx.Import().
func (sc simpleCtx) Import(importPath string, srcDir string, mode build.ImportMode) (*PackageData, error) {
	pkg, err := sc.bctx.Import(importPath, srcDir, mode)
	if err != nil {
		return nil, err
	}
	jsFiles, err := jsFilesFromDir(&sc.bctx, pkg.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate .inc.js files in %s: %w", pkg.Dir, err)
	}
	return &PackageData{
		Package:   pkg,
		IsVirtual: sc.isVirtual,
		JSFiles:   jsFiles,
	}, nil
}

// embeddedCtx creates simpleCtx that imports from a virtual FS embedded into
// the GopherJS compiler.
func embeddedCtx(embedded http.FileSystem, GOOS, GOARCH string) *simpleCtx {
	fs := &vfs{embedded}
	ec := simpleCtx{
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
		isVirtual: true,
	}
	return &ec
}

// goCtx creates simpleCtx that imports from the real file system GOROOT, GOPATH
// or Go Modules.
func goCtx(installSuffix string, buildTags []string) *simpleCtx {
	gc := simpleCtx{
		bctx: build.Context{
			GOROOT:        DefaultGOROOT,
			GOPATH:        build.Default.GOPATH,
			GOOS:          build.Default.GOOS,
			GOARCH:        "js",
			InstallSuffix: installSuffix,
			Compiler:      "gc",
			BuildTags: append(buildTags,
				"netgo",  // See https://godoc.org/net#hdr-Name_Resolution.
				"purego", // See https://golang.org/issues/23172.
			),
			CgoEnabled: true, // detect `import "C"` to throw proper error

			// go/build supports modules, but only when no FS access functions are
			// overridden and when provided ReleaseTags match those of the default
			// context (matching Go compiler's version).
			// This limitation is defined by the fact that it will invoke the Go tool
			// which can only see files on the real FS and will assume release tags
			// based on the Go tool's version.
			// TODO(nevkontakte): We should be able to omit this if we place
			// $GOROOT/bin at the front of $PATH.
			// See also: https://github.com/golang/go/issues/46856.
			ReleaseTags: build.Default.ReleaseTags[:compiler.GoVersion],
		},
	}
	return &gc
}

// IsPkgNotFound returns true if the error was caused by package not found.
//
// Unfortunately, go/build doesn't make use of typed errors, so we have to
// rely on the error message.
func IsPkgNotFound(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "cannot find package") || // Modules off.
			strings.Contains(err.Error(), "is not in GOROOT")) // Modules on.
}
