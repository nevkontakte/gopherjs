package main

import (
	"context"
	"flag"
	"fmt"
	"log"
)

type compilerFlags struct {
	commonFlags
	Output           string
	TrimPath         string
	Package          string
	Complete         bool
	BuildID          string
	GoVersion        string
	LocalImportPath  string
	ImportCfg        string
	Pack             bool
	Concurrency      int
	CompilingStd     bool
	CompilingRuntime bool
	SymABIs          string
	ASMHeader        string
}

func (cf *compilerFlags) Bind(tool string) *flag.FlagSet {
	fs := cf.commonFlags.Bind(tool)

	// The following flags mirror a subset of Go's cmd/compile flags used by the `go` tool.
	fs.StringVar(&cf.Output, "o", "",
		"Write object to file (default file.o or, with -pack, file.a).")
	fs.StringVar(&cf.TrimPath, "trimpath", "",
		"Remove prefix from recorded source file paths.")
	fs.StringVar(&cf.Package, "p", "",
		"Set expected package import path for the code being compiled, and diagnose imports that would cause a circular dependency.")
	fs.BoolVar(&cf.Complete, "complete", false,
		"Assume package has no non-Go components.")
	fs.StringVar(&cf.BuildID, "buildid", "",
		"Record id as the build id in the export metadata.")
	fs.StringVar(&cf.GoVersion, "goversion", "",
		"Specify required go tool version of the runtime. Exits when the runtime go version does not match goversion.")
	fs.StringVar(&cf.LocalImportPath, "D", "",
		"Set relative path for local imports.")
	fs.StringVar(&cf.ImportCfg, "importcfg", "",
		"Read import configuration from file. In the file, set importmap, packagefile to specify import resolution.")
	fs.BoolVar(&cf.Pack, "pack", false,
		"Write a package (archive) file rather than an object file.")
	fs.IntVar(&cf.Concurrency, "c", 1,
		"Concurrency during compilation. Set 1 for no concurrency (default is 1).")
	fs.BoolVar(&cf.CompilingStd, "std", false,
		"Compiling standard library.")
	fs.BoolVar(&cf.CompilingRuntime, "+", false,
		"Compiling runtime.")
	fs.StringVar(&cf.SymABIs, "symabis", "", "Read symbol ABIs from file.")
	fs.StringVar(&cf.ASMHeader, "asmhdr", "", "Write assembly header to file.")
	return fs
}

func compile(ctx context.Context, toolPath string, args ...string) error {
	flags := compilerFlags{}
	if err := flags.Bind("compile").Parse(args); err != nil {
		return fmt.Errorf("failed to parse compiler flags: %s", err)
	}
	if flags.ProcessSpecial() {
		return nil
	}

	log.Printf("%+v", flags)
	return nil
}
