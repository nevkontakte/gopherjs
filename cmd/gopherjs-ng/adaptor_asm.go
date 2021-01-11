package main

import (
	"context"
	"flag"
	"fmt"
	"log"
)

type asmFlags struct {
	commonFlags
	Output      string
	TrimPath    string
	Package     string
	IncludePath repeatedFlag
	Defines     repeatedFlag
	GenSymABIs  bool
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

	log.Printf("%+v", flags)
	return nil
}
