package main

import (
	"flag"
	"fmt"

	"github.com/gopherjs/gopherjs/compiler"
)

type commonFlags struct {
	Version string

	tool string
}

func (cf *commonFlags) Bind(tool string) *flag.FlagSet {
	cf.tool = tool

	fs := flag.NewFlagSet(tool, flag.ContinueOnError)
	fs.StringVar(&cf.Version, "V", cf.Version, "Print tool version and exit.")

	return fs
}

func (cf *commonFlags) ProcessSpecial() bool {
	if cf.Version == "full" {
		fmt.Println(cf.tool, "version", compiler.Version)
		return true
	}
	return false
}

// repeatedFlag allows passing multiple values to the flag by repeating it in the command line.
type repeatedFlag []string

func (rf *repeatedFlag) Set(value string) error {
	*rf = append(*rf, value)
	return nil
}

func (rf repeatedFlag) String() string {
	return fmt.Sprint([]string(rf))
}
