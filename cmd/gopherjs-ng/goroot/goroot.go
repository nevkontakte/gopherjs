// Package goroot is responsible for constructing and managing virtual GOROOT
// direcotry for GopherJS augmented standard library.
package goroot

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/gopherjs/gopherjs/cmd/gopherjs-ng/gotool"
	"github.com/gopherjs/gopherjs/compiler"
	"github.com/gopherjs/gopherjs/compiler/natives"
)

const (
	fileMode os.FileMode = 0644
	dirMode  os.FileMode = 0755
)

// Manager constructs and keeps augmented GOROOT path used by GopherJS.
//
// GopherJS ships with a set of standard library customizations required for the
// browser environment. To make these customizations visible to the go tool, we
// generate a virtual GOROOT directory, which contains augmented source codes
// for some of the packages and symlinks onto the original GOROOT for the rest.
//
// TODO: Use a fingerprint to only recreate GOROOT when Go or GopherJS version
// has changed.
type Manager struct {
	tool       gotool.GoTool
	realGoRoot string
	goVersion  string
}

// New Manager instance using the given Go tool.
func New(ctx context.Context, tool gotool.GoTool) *Manager {
	return &Manager{
		tool:       tool,
		realGoRoot: tool.GOROOT(ctx),
		goVersion:  tool.Version(ctx),
	}
}

func (m *Manager) vroot() string {
	return path.Join(os.TempDir(), fmt.Sprintf("goroot-gopherjs%s-%s", compiler.Version, m.goVersion))
}

// VirtualGOROOT sets up a GOROOT with GopherJS augmentations and returns its path.
func (m *Manager) VirtualGOROOT() (string, error) {
	vroot := m.vroot()
	// TODO: Only rebuild virtual GOROOT when necessary.
	if err := os.RemoveAll(vroot); err != nil {
		return "", fmt.Errorf("failed to delete stale virtual GOROOT: %w", err)
	}
	if err := os.MkdirAll(vroot, dirMode); err != nil {
		return "", fmt.Errorf("failed to virtual GOROOT: %w", err)
	}

	merger := &gorootMerger{
		overlayFS:   natives.FS,
		vanillaRoot: m.realGoRoot,
		mergedRoot:  vroot,
	}
	err := merger.dir(".")
	if err != nil {
		return "", fmt.Errorf("failed to build virtual GOROOT: %w", err)
	}
	// TODO: Link in github.com/gopherjs/gopherjs/{js,nosync} packages, since
	// certain standard library packages will depend on them.
	return vroot, nil
}
