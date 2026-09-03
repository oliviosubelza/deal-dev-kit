// Command deal-kit installs and updates shared development assets (skills,
// conventions, UI components) from the deal-dev-kit repository into a project.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/cli"
	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
)

// envOr returns the environment variable, or a fallback when it is unset.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

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
		kitDir       = fs.String("kit-dir", os.Getenv("DEAL_KIT_DIR"), "use a local kit checkout instead of fetching")
		repo         = fs.String("repo", envOr("DEAL_KIT_REPO", kit.DefaultRepo), "kit repository to fetch from")
		ref          = fs.String("ref", os.Getenv("DEAL_KIT_REF"), "kit tag, branch or SHA (default: newest kit-v* tag)")
		offline      = fs.Bool("offline", false, "use the cached kit without contacting the remote")
		typeOverride = fs.String("type", "", "force the project type instead of detecting it")
		yes          = fs.Bool("yes", false, "apply without confirmation")
		dryRun       = fs.Bool("dry-run", false, "print the plan and stop")
		noDeps       = fs.Bool("no-deps", false, "do not run the package manager")
		here         = fs.Bool("here", false, "use the current directory as the project root")
	)
	if err := fs.Parse(permute(fs, rest)); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	env := cli.Env{
		Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin,
		Cwd: cwd, Version: version,
		KitDir: *kitDir, Repo: *repo, Ref: *ref, Offline: *offline,
		AssumeYes: *yes, DryRun: *dryRun, NoDeps: *noDeps, Here: *here,
	}

	switch name {
	case "browse":
		return cli.Interactive(env, *typeOverride)
	case "init":
		return cli.Init(env, *typeOverride)
	case "add":
		return cli.Add(env, fs.Args())
	case "update":
		return cli.Update(env)
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
  update       Move the kit pin forward and re-sync
  status       Show what is installed, and whether it has drifted

Flags:
  --repo       Kit repository (or set DEAL_KIT_REPO)
  --ref        Kit tag, branch or SHA (or set DEAL_KIT_REF)
  --offline    Use the cached kit without contacting the remote
  --kit-dir    Use a local kit checkout instead of fetching (or DEAL_KIT_DIR)
  --type       Force the project type instead of detecting it
  --dry-run    Print the plan without writing anything
  --yes        Apply without confirmation
  --no-deps    Skip the package manager install
  --here       Use the current directory as the project root
`, version)
}
