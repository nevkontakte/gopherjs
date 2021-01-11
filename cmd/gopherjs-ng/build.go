package main

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
	goToolPath = flag.String("go_tool_path", path.Join(runtime.GOROOT(), "bin", "go"), "Path to a custom Go tool binary. For debugging only.")
)

func build(ctx context.Context, args ...string) error {
	return goTool(ctx, "build", args...)
}

func goTool(ctx context.Context, verb string, args ...string) error {
	bin := *goToolPath
	self := os.Args[0]

	args = append([]string{
		verb,
		"-toolexec=" + self + " adaptor", // Redirect toolchain calls back to ourselves.
		"-tags=" + strings.Join([]string{
			"netgo",  // See https://godoc.org/net#hdr-Name_Resolution.
			"purego", // See https://golang.org/issues/23172.
		}, ","),
	}, args...)

	cmd := exec.CommandContext(ctx, bin, args...)
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
