package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

type linkFlags struct {
	commonFlags
	Output    string
	ImportCfg string
	BuildMode string
	BuildID   string
	ExtLd     string
}

func (lf *linkFlags) Bind(tool string) *flag.FlagSet {
	fs := lf.commonFlags.Bind(tool)

	// The following flags mirror a subset of Go's cmd/asm flags used by the `go` tool.
	fs.StringVar(&lf.Output, "o", "",
		"Write object to file.")
	fs.StringVar(&lf.ImportCfg, "importcfg", "",
		"Read import configuration from file.")
	fs.StringVar(&lf.BuildMode, "buildmode", "",
		"Set build mode.")
	fs.StringVar(&lf.BuildID, "buildid", "",
		"Record id as Go toolchain build id.")
	fs.StringVar(&lf.ExtLd, "extld", "",
		"Use linker when linking in external mode.")
	return fs
}

func link(ctx context.Context, toolPath string, args ...string) error {
	flags := linkFlags{}
	if err := flags.Bind("link").Parse(args); err != nil {
		return fmt.Errorf("failed to parse link flags")
	}

	if flags.ProcessSpecial() {
		return nil
	}

	// TODO: Invoke GopherJS linker.

	f, err := os.Create(flags.Output)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", flags.Output, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "/*!gopherjs  \xff Go build ID: %q\n \xff */", flags.BuildID); err != nil {
		return fmt.Errorf("failed to write build id into the linker output: %w", err)
	}

	return nil
}
