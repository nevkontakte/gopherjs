package goroot

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// gorootMerger traverses both vanilla overlay GOROOT sources and generates a
// merged version in the specified directory.
//
// The resulting directory can be used as GOROOT for GopherJS builds and should
// be for the most part compatible with any normal Go tooling.
type gorootMerger struct {
	overlayFS   http.FileSystem
	vanillaRoot string
	mergedRoot  string
}

// dir recursively merges the specified path relative to GOROOT.
func (m *gorootMerger) dir(dir string) error {
	vanillaDir := abs(path.Join(m.vanillaRoot, dir))
	mergedDir := abs(path.Join(m.mergedRoot, dir))
	overlayDir := path.Clean(path.Join("/", dir))

	o, err := m.overlayFS.Open(overlayDir)
	if os.IsNotExist(err) {
		// The directory doesn't exist in the overlay, so we can simply symlink it
		// from the original GOROOT and avoid traversing it entirely.
		if err := os.Symlink(vanillaDir, mergedDir); err != nil {
			return fmt.Errorf("failed to symlink %q in the virtual GOROOT: %w", dir, err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to open %q in overlays: %w", dir, err) // Should never happen.
	}
	defer o.Close()

	// If we are here, either this directory, or one of its children has
	// GopherJS-specific augmentations. We need to create it in the virtual GOROOT,
	// possible apply augmentations and then traverse all children directories.

	if err := os.MkdirAll(mergedDir, dirMode); err != nil {
		return fmt.Errorf("failed to create %q in virtual GOROOT: %w", dir, err)
	}

	vanillaEntries, err := ioutil.ReadDir(vanillaDir)
	if err != nil {
		return fmt.Errorf("failed to enumerate files in %q: %w", vanillaDir, err)
	}
	overlayEntries, err := o.Readdir(0)
	if err != nil {
		return fmt.Errorf("failed to enumerate files in %q: %w", overlayDir, err)
	}

	// Apply GopherJS augmentations to the original sources.
	if err := m.augmentPackage(dir, onlyFiles(vanillaEntries), onlyFiles(overlayEntries)); err != nil {
		return fmt.Errorf("failed to augment %q: %w", dir, err)
	}

	// Now traverse all subdirectories and merge them in the same way.
	for _, child := range onlyDirs(vanillaEntries) {
		if err := m.dir(path.Join(dir, child.Name())); err != nil {
			return err
		}
	}
	return nil
}

// augmentPackage processes sources in the given GOROOT directory and generates
// a GopherJS-compatible version in the corresponding merged GOROOT subdirectory.
func (m *gorootMerger) augmentPackage(dir string, vanilla []os.FileInfo, overlay []os.FileInfo) error {
	mergedDir := path.Join(m.mergedRoot, dir)
	overlayDir := path.Clean(path.Join("/", dir))

	// Phase 1: Collect the list of symbols we will be replacing and write out
	// our augmentation source files into the merged GOROOT.

	sf := SymbolFilter{}
	for _, n := range overlay {
		loadPath := filepath.Join(overlayDir, n.Name())
		writePath := filepath.Join(mergedDir, "gopherjs__"+n.Name()) // Avoid conflicts with original sources.
		if err := processSource(m.overlayFS, loadPath, writePath, sf.Collect); err != nil {
			return fmt.Errorf("failed to process augmentation source %q: %w", loadPath, err)
		}
	}

	// Phase 2: Filter the list of vanilla sources at file level.
	vanilla = onlyGoSources(vanilla) // GopherJS doesn't support Assembly.
	if filter, ok := extraFilters[dir]; ok {
		vanilla = filter(vanilla)
	}

	// Phase 3: Process the remaining original sources, prune augmented symbols
	// and write them out into the virtual GOROOT.
	for _, o := range vanilla {
		loadFS := http.Dir(m.vanillaRoot) // Read from the real file system.
		loadPath := filepath.Join(dir, o.Name())
		writePath := filepath.Join(mergedDir, o.Name())
		if err := processSource(loadFS, loadPath, writePath, sf.Prune); err != nil {
			return fmt.Errorf("failed to process original source %q: %w", loadPath, err)
		}
	}

	return nil
}

type fileFilter func(in []os.FileInfo) []os.FileInfo

func onlyDirs(in []os.FileInfo) []os.FileInfo {
	out := []os.FileInfo{}
	for _, e := range in {
		if e.IsDir() {
			out = append(out, e)
		}
	}
	return out
}

func onlyFiles(in []os.FileInfo) []os.FileInfo {
	out := []os.FileInfo{}
	for _, e := range in {
		if e.Name() == ".gitkeep" {
			// Special case, ensures that testdata/empty_overlay/overlay exists in Git,
			// but should be ignored otherwise.
			continue
		}
		if e.IsDir() {
			continue
		}
		out = append(out, e)
	}
	return out
}

func onlyGoSources(in []os.FileInfo) []os.FileInfo {
	out := []os.FileInfo{}
	for _, e := range in {
		if strings.HasSuffix(e.Name(), ".go") {
			out = append(out, e)
		}
	}
	return out
}

// TODO: Populate filters.
var extraFilters map[string]fileFilter = map[string]fileFilter{}

func abs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		panic("failed to get absolute path of: " + p)
	}
	return a
}
