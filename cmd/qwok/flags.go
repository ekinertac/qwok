// flags.go: tiny argument-parsing helpers shared by the subcommands.
//
// Convention (matches every usage string): the app name is the first token after
// the subcommand, before any flags — `qwok <cmd> <name> [flags...]`. This keeps
// parsing trivial and predictable; Go's flag package stops at the first
// positional, so a fixed name-first order avoids ambiguity with flag values
// (e.g. --cmd "npm run dev", whose value also doesn't start with '-').
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	return fs
}

// takeName pulls the leading positional app name (if present) and returns the
// remaining args for flag parsing.
func takeName(_ *flag.FlagSet, args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}
