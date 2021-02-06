package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path"

	"github.com/gopherjs/gopherjs/cmd/gopherjs-ng/goroot"
	"github.com/gopherjs/gopherjs/cmd/gopherjs-ng/gotool"
)

func run(ctx context.Context) error {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		return fmt.Errorf("command verb not specified")
	}

	tool, err := gotool.Discover()
	if err != nil {
		return err
	}

	verb, args := args[0], args[1:]

	switch verb {
	case "adaptor":
		return adaptor(ctx, args...)
	case "build", "test", "install":
		return tool.Run(ctx, verb, args...)
	case "vroot":
		vroot, err := goroot.New(ctx, tool).VirtualGOROOT()
		if err != nil {
			return fmt.Errorf("failed to set up virtual GOROOT: %w", err)
		}
		fmt.Println(vroot)
		return nil
	default:
		return fmt.Errorf("unknown command verb %q", verb)
	}
}

func adaptor(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing positional argument for tool executable")
	}

	switch tool := path.Base(args[0]); tool {
	case "compile":
		return compile(ctx, args[0], args[1:]...)
	case "asm":
		return asm(ctx, args[0], args[1:]...)
	case "link":
		return link(ctx, args[0], args[1:]...)
	default:
		return fmt.Errorf("unimplemented tool %q: %v", tool, args)
	}
}

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatalf("Fatal error: %s", err)
	}
}
