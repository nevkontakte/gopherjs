package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

type asmFlags struct {
	commonFlags
	Output           string
	TrimPath         string
	Package          string
	IncludePath      repeatedFlag
	Defines          repeatedFlag
	GenSymABIs       bool
	CompilingRuntime bool
}

func (af *asmFlags) Bind(tool string) *flag.FlagSet {
	fs := af.commonFlags.Bind(tool)

	// The following flags mirror a subset of Go's cmd/asm flags used by the `go` tool.
	fs.StringVar(&af.Output, "o", "",
		"Write object to file (default file.o or, with -pack, file.a).")
	fs.StringVar(&af.TrimPath, "trimpath", "",
		"Remove prefix from recorded source file paths.")
	fs.StringVar(&af.Package, "p", "",
		"Set expected package import path for the code being compiled.")
	fs.Var(&af.IncludePath, "I",
		"Include directory; can be set multiple times.")
	fs.Var(&af.Defines, "D",
		"Predefined symbol with optional simple value -D=identifier=value; can be set multiple times.")
	fs.BoolVar(&af.GenSymABIs, "gensymabis", false,
		"Write symbol ABI information to output file, don't assemble.")
	fs.BoolVar(&af.CompilingRuntime, "compiling-runtime", false,
		"Source to be compiled is part of the Go runtime")
	return fs
}

func asm(ctx context.Context, toolPath string, args ...string) error {
	flags := asmFlags{}
	if err := flags.Bind("asm").Parse(args); err != nil {
		return fmt.Errorf("failed to parse asm flags")
	}

	if flags.ProcessSpecial() {
		return nil
	}

	// TODO: GopherJS doesn't really support Go assembly. For now we just create
	// an empty output file to make the Go tool happy, but ideally we should avoid
	// invoking asm tool in the first place.

	f, err := os.Create(flags.Output)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", flags.Output, err)
	}
	f.Close()

	return nil
}
