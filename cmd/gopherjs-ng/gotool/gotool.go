// Package gotool provides helpers for interacting with the go tool.
package gotool

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

var (
	goToolPath = flag.String("go_tool_path", "", "Override `go` tool executable path. By default, the executable is searched for in the $GOROOT and $PATH.")
)

var runtimeGOROOT = runtime.GOROOT // for tests

// GoTool provides a higher-level interface for the go tool.
type GoTool struct {
	Path string
}

// Discover GoTool instance. Executable path is auto-detected based on the environment or the flags.
func Discover() (GoTool, error) {
	// TODO: Consider calling `go version` to verify that the tool actually works.
	if !flag.Parsed() {
		return GoTool{}, fmt.Errorf("flag.Parse() must be called before gotool.New()")
	}

	if *goToolPath != "" {
		if _, err := os.Stat(*goToolPath); err != nil {
			return GoTool{}, fmt.Errorf("go tool path %q is invalid: %w", *goToolPath, err)
		}
		return GoTool{Path: *goToolPath}, nil
	}

	inGoRoot := path.Join(runtimeGOROOT(), "bin", "go")
	if _, err := os.Stat(inGoRoot); err == nil {
		return GoTool{Path: inGoRoot}, nil
	}

	inPath, err := exec.LookPath("go")
	if err == nil {
		return GoTool{Path: inPath}, nil
	}

	return GoTool{}, fmt.Errorf("go tool is not found (GOROOT=%q, PATH=%q)", runtime.GOROOT(), os.Getenv("PATH"))
}

// Run go tool with the given subcommand and arguments.
//
// Go tool will be configured correctly with the GopherJS GOOS/GOARCH and tags,
// and build toolchain invocations will be intercepted by the GopherJS binary.
func (t GoTool) Run(ctx context.Context, subcmd string, args ...string) error {
	self := os.Args[0]

	args = append([]string{
		subcmd,
		"-toolexec=" + self + " adaptor", // Redirect toolchain calls back to ourselves.
		"-tags=" + strings.Join([]string{
			"netgo",            // See https://godoc.org/net#hdr-Name_Resolution.
			"purego",           // See https://golang.org/issues/23172.
			"math_big_pure_go", // Avoid using any assembly in math/big package.
		}, ","),
	}, args...)

	cmd := exec.CommandContext(ctx, t.Path, args...)
	cmd.Env = append(os.Environ(),
		"GOOS=js",
		"GOARCH=js",
		"CGO_ENABLED=0",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go tool invocation failed: %w", err)
	}
	return nil
}

// GOROOT returns the result of `go env GOROOT`, or panics if execution fails.
func (t GoTool) GOROOT(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, t.Path, "env", "GOROOT")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		panic(fmt.Sprintf("%s failed: %s", cmd, err)) // This should never happen.
	}
	// TODO: Cache result.
	return strings.TrimRight(string(output), "\r\n")
}

// Version of the Go toolchain in go1.x.y format.
func (t GoTool) Version(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, t.Path, "version")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		panic(fmt.Sprintf("%s failed: %s", cmd, err)) // This should never happen.
	}
	outputStr := string(output)
	parts := strings.Split(outputStr, " ")
	if len(parts) != 4 {
		panic(fmt.Sprintf("unexpected `go version` output: %q", outputStr))
	}
	// TODO: Cache result.
	return parts[2]
}
