package build

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"
)

// gopherjsBuildTime is the build time of the compiler executable itself.
var gopherjsBuildTime time.Time

func init() {
	// Determine when GopherJS compiler was built, this will be required to decide
	// whether existing pre-compiler archives can be reused during the build.
	// If for any reason we can't determine that, it's safe to assume GopherJS was
	// built just now.
	gopherjsBuildTime = time.Now()
	path, err := os.Executable()
	if err != nil {
		return
	}
	stat, err := os.Stat(path)
	if err != nil {
		return
	}
	gopherjsBuildTime = stat.ModTime()
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		panic(fmt.Errorf("failed to get absolute path to %s", p))
	}
	return a
}

// makeWritable attempts to make the given path writable by its owner.
func makeWritable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	err = os.Chmod(path, info.Mode()|0700)
	if err != nil {
		return err
	}
	return nil
}

// pkgModTime determines most recent package source modification time.
//
// If for any reason the time can't be determined, time.Now() is assumed as if
// it was just modified.
func pkgModTime(pkg *PackageData, statFn func(path string) (fs.FileInfo, error)) time.Time {
	if statFn == nil {
		statFn = os.Stat
	}

	t := gopherjsBuildTime
	for _, f := range pkg.Sources() {
		finfo, err := statFn(path.Join(pkg.Dir, f))
		if err != nil {
			return time.Now()
		}
		if finfo.ModTime().After(t) {
			t = finfo.ModTime()
		}
	}
	return t
}
