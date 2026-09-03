package main

import (
	"flag"
	"sort"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/cli"
)

// command is one subcommand: its name, its one-line help, and how to run it.
//
// The table is the single source of truth for both dispatch and name
// recognition. Keeping a separate list of valid names let the two drift, and
// twice a new command silently fell through to the interactive browser.
type command struct {
	name    string
	summary string
	run     func(env cli.Env, fs *flag.FlagSet, flags *commandFlags) error
}

// commandFlags holds the parsed flags a command may read.
type commandFlags struct {
	typeOverride string
	check        bool
}

func commands() []command {
	return []command{
		{"browse", "Open the interactive browser",
			func(e cli.Env, _ *flag.FlagSet, f *commandFlags) error { return cli.Interactive(e, f.typeOverride) }},
		{"new", "Create a project with its official generator, then install",
			func(e cli.Env, fs *flag.FlagSet, f *commandFlags) error { return cli.New(e, fs.Arg(0), f.typeOverride) }},
		{"init", "Set up this project: detect its type and install its profile",
			func(e cli.Env, _ *flag.FlagSet, f *commandFlags) error { return cli.Init(e, f.typeOverride) }},
		{"add", "Install additional artifacts",
			func(e cli.Env, fs *flag.FlagSet, _ *commandFlags) error { return cli.Add(e, fs.Args()) }},
		{"update", "Move the kit pin forward and re-sync",
			func(e cli.Env, _ *flag.FlagSet, _ *commandFlags) error { return cli.Update(e) }},
		{"status", "Show what is installed, and whether it has drifted",
			func(e cli.Env, _ *flag.FlagSet, _ *commandFlags) error { return cli.Status(e) }},
		{"doctor", "Report which external tools are installed",
			func(e cli.Env, _ *flag.FlagSet, _ *commandFlags) error { return cli.Doctor(e) }},
		{"self-update", "Replace this binary with the latest release",
			func(e cli.Env, _ *flag.FlagSet, f *commandFlags) error { return cli.SelfUpdate(e, f.check) }},
	}
}

// lookup finds a command by name.
func lookup(name string) (command, bool) {
	for _, c := range commands() {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// commandNames are the recognised subcommand names, derived from the table so
// the two can never disagree.
func commandNames() map[string]bool {
	out := make(map[string]bool, len(commands()))
	for _, c := range commands() {
		out[c.name] = true
	}
	return out
}

// visibleCommands are the ones listed in the usage text. "browse" is what
// running deal-kit with no subcommand does, so it is shown as "(none)".
func visibleCommands() []command {
	cs := commands()
	sort.SliceStable(cs, func(i, j int) bool { return false })
	return cs
}
