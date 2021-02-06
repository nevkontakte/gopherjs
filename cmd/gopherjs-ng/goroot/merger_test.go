package goroot

import (
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMerger(t *testing.T) {
	// These test cases cover various GOROOT merging scenarios and are devined by
	// fixtures in the testdata subdirectory. Each test case is specified by three
	// subdirectories:
	//
	//  - `vanilla` — fake "original" GOROOT.
	//  - `overlay` — fake "overlay" GOROOT with GopherJS augmentations.
	//  - `want` — expected result of merging the two.
	tests := []string{
		// The vanilla package is entirely absent from the overlay and should be
		// copied to the merged GOROOT unmodified.
		"empty_overlay",
		// The vanilla package is present in the overlay tree and actually has
		// augmentations. The merged GOROOT should contain original sources with
		// augmented symbols removed, and overlay files added to the package.
		"simple_augmentation",
		// Overlay contains only augmentations for a subpackage, so the parent
		// package sources should be copied over unmodified.
		"subpackage_augmentation",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			got := t.TempDir()
			want := path.Join("testdata", test, "want")
			m := &gorootMerger{
				overlayFS:   http.Dir(abs(path.Join("testdata", test, "overlay"))),
				vanillaRoot: abs(path.Join("testdata", test, "vanilla")),
				mergedRoot:  got,
			}
			if err := m.dir("."); err != nil {
				t.Errorf("m.dir() returned error: %s", err)
			}
			if diff := cmp.Diff(loadDir(t, want), loadDir(t, got)); diff != "" {
				t.Errorf("m.dir() produced diff (-want,+got):\n%s", diff)
			}
		})
	}
}

// loadDir recursively traverses a directory and returns its contents in a map
// which can be used for diffing.
//
// The map is keyed with paths relative to the passed root directory and values
// are file content for regular files and dummy "directory exists" strings for
// directories. The latter makes sure that empty directories are represented in
// the return value.
//
// Symlinks are resolved and traversed as if they were regular files to make the
// test agnostic of symlink-based optimizations we might actually use.
func loadDir(t *testing.T, root string) map[string]string {
	t.Helper()
	content := map[string]string{}
	rel := func(p string) string { return strings.TrimPrefix(p, root) }
	var walk func(dir string)
	walk = func(dir string) {
		t.Helper()
		for _, entry := range readdir(t, dir) {
			path := filepath.Join(dir, entry.Name())
			if stat(t, path).IsDir() {
				content[rel(path)] = "directory exists"
				walk(path)
			} else {
				content[rel(path)] = readall(t, path)
			}
		}
	}
	walk(root)
	return content
}

func readdir(t *testing.T, dir string) []os.FileInfo {
	t.Helper()
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to enumerate %q: %s", dir, err)
	}
	return entries
}

func readall(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open %q: %s", path, err)
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Failed to read %q: %s", path, err)
	}
	return string(data)
}

func stat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat %q: %s", path, err)
	}
	return info
}
