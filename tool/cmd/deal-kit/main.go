// Command deal-kit installs and updates shared development assets (skills,
// conventions, UI components) from the deal-dev-kit repository into a project.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/deal/deal-dev-kit/tool/internal/cli"
)

// version is injected at build time via -ldflags.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, cli.ErrAborted) {
			fmt.Fprintln(os.Stderr, "aborted")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if help, showVersion := isHelpOrVersion(args); help || showVersion {
		if help {
			usage()
		} else {
			fmt.Println(version)
		}
		return nil
	}

	// No subcommand opens the interactive browser.
	name, rest := extractCommand(args)
	fs := flag.NewFlagSet("deal-kit "+name, flag.ContinueOnError)
	var (
		kitDir       = fs.String("kit-dir", os.Getenv("DEAL_KIT_DIR"), "path to a checkout of the kit repository")
		typeOverride = fs.String("type", "", "force the project type instead of detecting it")
		yes          = fs.Bool("yes", false, "apply without confirmation")
		dryRun       = fs.Bool("dry-run", false, "print the plan and stop")
		noDeps       = fs.Bool("no-deps", false, "do not run the package manager")
	)
	if err := fs.Parse(permute(fs, rest)); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if *kitDir == "" {
		return errors.New("no kit directory: pass --kit-dir or set DEAL_KIT_DIR")
	}

	env := cli.Env{
		Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin,
		Cwd: cwd, KitDir: *kitDir, Version: version,
		AssumeYes: *yes, DryRun: *dryRun, NoDeps: *noDeps,
	}

	switch name {
	case "browse":
		return cli.Interactive(env, *typeOverride)
	case "init":
		return cli.Init(env, *typeOverride)
	case "add":
		return cli.Add(env, fs.Args())
	case "status":
		return cli.Status(env)
	default:
		usage()
		return fmt.Errorf("unknown command %q", name)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `deal-kit %s

Usage:
  deal-kit <command> [flags]

Commands:
  (none)       Open the interactive browser
  init         Set up this project: detect its type and install its profile
  add <id>...  Install additional artifacts
  status       Show what is installed, and whether it has drifted

Flags:
  --kit-dir    Path to a kit checkout (or set DEAL_KIT_DIR)
  --type       Force the project type instead of detecting it
  --dry-run    Print the plan without writing anything
  --yes        Apply without confirmation
  --no-deps    Skip the package manager install
`, version)
}
