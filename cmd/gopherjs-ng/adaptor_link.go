package main

import (
	"context"
	"fmt"
	"log"
)

func link(ctx context.Context, toolPath string, args ...string) error {
	flags := commonFlags{}
	if err := flags.Bind("link").Parse(args); err != nil {
		return fmt.Errorf("failed to parse asm flags")
	}

	if flags.ProcessSpecial() {
		return nil
	}

	log.Printf("%+v", flags)
	return nil
}
