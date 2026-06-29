// Command qwok is the CLI entry point: it parses arguments and dispatches to the
// orchestration layer (internal/app), then formats results for the terminal.
//
// It is deliberately thin — all policy lives in internal/app so it can be tested
// without a TTY. Subcommands map 1:1 to app functions:
//   add · run · list/ls · stop · kill · restart · logs · open · rm
//
// See AGENTS.md and docs/superpowers/specs/2026-06-26-qwok-design.md for the
// architecture this serves.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/ekinertac/qwok/internal/app"
)

const usage = `qwok — run your local dev apps by name

Usage:
  qwok add <name> --cmd "<command>" [--app] [--cwd <dir>] [--app-port N] [--env K=V ...]
                                 (--app = desktop/non-web: run directly, no portless/URL)
  qwok run [<name>] [--force]    (no name: infer from .qwok.toml in the current dir)
  qwok list                       (alias: ls)
  qwok stop <name>                graceful SIGTERM
  qwok kill <name>                forceful SIGKILL
  qwok restart <name>
  qwok logs <name> [-f]
  qwok open <name>                open https://<name>.localhost
  qwok rm <name> [--keep-file]
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "add":
		err = cmdAdd(args)
	case "run":
		err = cmdRun(args)
	case "list", "ls":
		err = cmdList(args)
	case "stop":
		err = cmdSignal(args, "stop")
	case "kill":
		err = cmdSignal(args, "kill")
	case "restart":
		err = cmdRestart(args)
	case "logs":
		err = cmdLogs(args)
	case "open":
		err = cmdOpen(args)
	case "rm", "remove":
		err = cmdRemove(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// envFlag collects repeated --env K=V pairs.
type envFlag map[string]string

func (e envFlag) String() string { return "" }
func (e envFlag) Set(v string) error {
	k, val, ok := strings.Cut(v, "=")
	if !ok {
		return fmt.Errorf("expected K=V, got %q", v)
	}
	e[k] = val
	return nil
}

func cmdAdd(args []string) error {
	fs := newFlagSet("add")
	cwd, _ := os.Getwd()
	dir := fs.String("cwd", cwd, "project directory")
	cmd := fs.String("cmd", "", "dev command to run (required)")
	asApp := fs.Bool("app", false, "desktop/non-web app: run directly, no portless or URL")
	port := fs.Int("app-port", 0, "pin a fixed port (web apps only; default: portless auto-assigns)")
	force := fs.Bool("force", false, "overwrite an existing registration")
	env := envFlag{}
	fs.Var(env, "env", "extra environment as K=V (repeatable)")
	name, rest := takeName(fs, args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" || *cmd == "" {
		return fmt.Errorf("usage: qwok add <name> --cmd \"<command>\" [--app] [--cwd <dir>]")
	}
	typ := "web"
	if *asApp {
		typ = "app"
	}
	if err := app.Add(app.AddOptions{
		Name: name, Cwd: *dir, Cmd: *cmd, Type: typ, AppPort: *port, Env: env, Force: *force,
	}); err != nil {
		return err
	}
	fmt.Printf("registered %q -> %s\n", name, *dir)
	return nil
}

func cmdRun(args []string) error {
	fs := newFlagSet("run")
	force := fs.Bool("force", false, "restart if already running")
	name, rest := takeName(fs, args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		// No name: infer the app from the nearest .qwok.toml (run from a project dir).
		n, err := app.LocalName()
		if err != nil {
			return err
		}
		name = n
	}
	url, err := app.Run(name, *force)
	if err != nil {
		return err
	}
	if url == "" { // app-type: no URL to show
		fmt.Printf("started %q\n", name)
	} else {
		fmt.Printf("started %q -> %s\n", name, url)
	}
	return nil
}

func cmdList(_ []string) error {
	rows, err := app.List()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("no apps registered yet — add one with: qwok add <name> --cmd \"<command>\"")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tSTATUS\tURL\tPORT")
	for _, s := range rows {
		status := "stopped"
		port := ""
		if s.Running {
			status = "running"
			if s.Port > 0 {
				port = fmt.Sprintf("%d", s.Port)
			}
		}
		url := s.URL
		if url == "" { // app-type has no URL
			url = "—"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Type, status, url, port)
	}
	return w.Flush()
}

func cmdSignal(args []string, which string) error {
	fs := newFlagSet(which)
	name, rest := takeName(fs, args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("usage: qwok %s <name>", which)
	}
	var err error
	past := "stopped"
	if which == "kill" {
		err = app.Kill(name)
		past = "killed"
	} else {
		err = app.Stop(name)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s %q\n", past, name)
	return nil
}

func cmdRestart(args []string) error {
	fs := newFlagSet("restart")
	name, rest := takeName(fs, args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("usage: qwok restart <name>")
	}
	url, err := app.Restart(name)
	if err != nil {
		return err
	}
	fmt.Printf("restarted %q -> %s\n", name, url)
	return nil
}

func cmdLogs(args []string) error {
	fs := newFlagSet("logs")
	follow := fs.Bool("f", false, "follow the log (tail -f)")
	name, rest := takeName(fs, args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("usage: qwok logs <name> [-f]")
	}
	path, err := app.LogPath(name)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no logs yet for %q (has it been run?)", name)
	}
	tailArgs := []string{"-n", "+1"}
	if *follow {
		tailArgs = append(tailArgs, "-f")
	}
	tailArgs = append(tailArgs, path)
	c := exec.Command("tail", tailArgs...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}

func cmdOpen(args []string) error {
	fs := newFlagSet("open")
	name, rest := takeName(fs, args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("usage: qwok open <name>")
	}
	url, err := app.URL(name)
	if err != nil {
		return err
	}
	return exec.Command("open", url).Run()
}

func cmdRemove(args []string) error {
	fs := newFlagSet("rm")
	keep := fs.Bool("keep-file", false, "keep the project's .qwok.toml")
	name, rest := takeName(fs, args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("usage: qwok rm <name>")
	}
	if err := app.Remove(name, *keep); err != nil {
		return err
	}
	fmt.Printf("removed %q\n", name)
	return nil
}
