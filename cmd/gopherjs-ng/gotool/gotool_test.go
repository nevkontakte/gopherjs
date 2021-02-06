package gotool

import (
	"context"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
)

func TestDiscovery(t *testing.T) {
	t.Run("from_flag", func(t *testing.T) {
		defer restoreFlag(goToolPath)()
		want := makeGoTool(t, "go_from_flag")
		*goToolPath = want

		tool := mustDiscover(t)
		if tool.Path != want {
			t.Errorf("Discover().Path is %q, want %q.", tool.Path, want)
		}
	})

	t.Run("from_goroot", func(t *testing.T) {
		defer func() { runtimeGOROOT = runtime.GOROOT }()
		want := makeGoTool(t, "goroot/bin/go")
		// Can't use os.Setenv() because runtime package captures GOROOT env at init time.
		runtimeGOROOT = func() string { return path.Dir(path.Dir(want)) }

		tool := mustDiscover(t)
		if tool.Path != want {
			t.Errorf("Discover().Path is %q, want %q.", tool.Path, want)
		}
	})

	t.Run("from_path", func(t *testing.T) {
		defer func() { runtimeGOROOT = runtime.GOROOT }()
		defer restoreEnv()()
		want := makeGoTool(t, "go")
		runtimeGOROOT = func() string { return path.Join(t.TempDir(), "does_not_exist") }
		os.Setenv("PATH", path.Dir(want))

		tool := mustDiscover(t)
		if tool.Path != want {
			t.Errorf("Discover().Path is %q, want %q.", tool.Path, want)
		}
	})
}

func TestRun(t *testing.T) {
	t.Skip("Testing GoTool.Run() without side effects is currently impossible.")
}

func TestGOROOT(t *testing.T) {
	defer restoreEnv()()
	os.Setenv("GOPATH", t.TempDir()) // Doesn't matter which, as long as it's set.
	os.Setenv("GOCACHE", t.TempDir())

	tool := mustDiscover(t)

	if want := path.Join(runtime.GOROOT(), "bin", "go"); tool.Path != want {
		t.Errorf("Discovered Go tool %q, want %q", tool.Path, want)
	}

	if got, want := tool.GOROOT(context.Background()), runtime.GOROOT(); got != want {
		t.Errorf("tool.GOROOT() returned %q, want %q", got, want)
	}
}

func TestVersion(t *testing.T) {
	defer restoreEnv()()
	os.Setenv("GOPATH", t.TempDir()) // Doesn't matter which, as long as it's set.

	tool := mustDiscover(t)

	if want := path.Join(runtime.GOROOT(), "bin", "go"); tool.Path != want {
		t.Errorf("Discovered Go tool %q, want %q", tool.Path, want)
	}

	if got, want := tool.Version(context.Background()), runtime.Version(); got != want {
		t.Errorf("tool.Version() returned %q, want %q", got, want)
	}
}

func makeGoTool(t *testing.T, newname string) string {
	t.Helper()
	newname = path.Join(t.TempDir(), newname)
	oldname := path.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(oldname); err != nil {
		t.Fatalf("Expected go tool to exist at %q: %s", oldname, err)
	}
	if err := os.MkdirAll(path.Dir(newname), 0755); err != nil {
		t.Fatalf("Failed to create directory %q: %s", path.Dir(newname), err)
	}
	if err := os.Symlink(oldname, newname); err != nil {
		t.Fatalf("Failed to create a symlink: %s", err)
	}
	return newname
}

func restoreFlag(val *string) func() {
	captured := *val
	return func() { *val = captured }
}

func restoreEnv() func() {
	captured := []string{}
	copy(captured, os.Environ()) // Make sure only we have a reference to captured.

	return func() {
		os.Clearenv()
		for _, line := range captured {
			parts := strings.SplitN(line, "=", 2)
			os.Setenv(parts[0], parts[1])
		}
	}
}

func mustDiscover(t *testing.T) GoTool {
	t.Helper()

	tool, err := Discover()
	if err != nil {
		t.Fatalf("Discover() returned error %q, want no error", err)
	}
	return tool
}
