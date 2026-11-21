// Command flakehash recomputes fixed-output-derivation hashes (vendorHash,
// npmDepsHash, pnpmDeps.hash, ...) in Nix files after an upstream dependency
// manifest (go.sum, package.json, pnpm-lock.yaml, ...) changed.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"git.myservermanager.com/varakh/flakehash/internal/meta"
	"git.myservermanager.com/varakh/flakehash/internal/nixhash"
)

type fileList []string

func (f *fileList) String() string { return strings.Join(*f, ",") }

func (f *fileList) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	var files fileList
	flag.Var(&files, "file", "nix file to patch, repeatable (default: flake.nix)")
	maxIterations := flag.Int("max-iterations", 10, "max rebuild attempts per target")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: flakehash [--file flake.nix] [--max-iterations 10] <target> [target...]")
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s\n", meta.Name, meta.Version)
		return
	}

	targets := flag.Args()
	if len(targets) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if len(files) == 0 {
		files = fileList{"flake.nix"}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	anyChanged := false
	for _, target := range targets {
		changed, err := nixhash.FixTarget(ctx, target, files, *maxIterations, nixhash.Build)
		anyChanged = anyChanged || changed
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", target, err)
			os.Exit(1)
		}
	}

	if !anyChanged {
		fmt.Println("all hashes already correct")
	}
}
