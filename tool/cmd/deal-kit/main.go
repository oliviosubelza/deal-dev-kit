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
	cmd, ok := lookup(name)
	if !ok {
		usage()
		return fmt.Errorf("unknown command %q", name)
	}

	fs := flag.NewFlagSet("deal-kit "+name, flag.ContinueOnError)
	var (
		kitDir  = fs.String("kit-dir", os.Getenv("DEAL_KIT_DIR"), "use a local kit checkout instead of fetching")
		repo    = fs.String("repo", envOr("DEAL_KIT_REPO", kit.DefaultRepo), "kit repository to fetch from")
		ref     = fs.String("ref", os.Getenv("DEAL_KIT_REF"), "kit tag, branch or SHA (default: newest kit-v* tag)")
		offline = fs.Bool("offline", false, "use the cached kit without contacting the remote")
		yes     = fs.Bool("yes", false, "apply without confirmation")
		dryRun  = fs.Bool("dry-run", false, "print the plan and stop")
		noDeps  = fs.Bool("no-deps", false, "do not run the package manager")
		here    = fs.Bool("here", false, "use the current directory as the project root")

		flags commandFlags
	)
	fs.StringVar(&flags.typeOverride, "type", "", "force the project type instead of detecting it")
	fs.BoolVar(&flags.check, "check", false, "self-update: report the latest version without installing it")

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
		ReleaseRepo: os.Getenv("DEAL_KIT_RELEASE_REPO"),
		AssumeYes:   *yes, DryRun: *dryRun, NoDeps: *noDeps, Here: *here,
	}
	return cmd.run(env, fs, &flags)
}

// envOr returns the environment variable, or a fallback when it is unset.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func usage() {
	fmt.Fprintf(os.Stderr, "deal-kit %s\n\nUsage:\n  deal-kit <command> [flags]\n\nCommands:\n", version)
	for _, c := range visibleCommands() {
		label := c.name
		if c.name == "browse" {
			label = "(none)"
		}
		if c.name == "add" {
			label = "add <id>..."
		}
		if c.name == "new" {
			label = "new <dir>"
		}
		fmt.Fprintf(os.Stderr, "  %-13s %s\n", label, c.summary)
	}
	fmt.Fprint(os.Stderr, `
Flags:
  --repo       Kit repository (or set DEAL_KIT_REPO)
  --ref        Kit tag, branch or SHA (or set DEAL_KIT_REF)
  --offline    Use the cached kit without contacting the remote
  --kit-dir    Use a local kit checkout instead of fetching (or DEAL_KIT_DIR)
  --type       Force the project type instead of detecting it
  --here       Use the current directory as the project root
  --dry-run    Print the plan without writing anything
  --yes        Apply without confirmation
  --no-deps    Skip the package manager install
  --check      With self-update, only report what is available
`)
}
