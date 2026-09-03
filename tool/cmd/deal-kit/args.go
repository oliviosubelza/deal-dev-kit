package main

import (
	"flag"
	"strings"
)

// permute reorders args so every flag comes before every positional argument.
//
// Go's flag package stops parsing at the first non-flag argument, so
// `deal-kit add ui-kit/data-table --kit-dir /path` would silently ignore
// --kit-dir. Users write flags in whatever order reads naturally, so accept
// both and hand flag.Parse the order it needs.
func permute(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Everything after a bare "--" is positional by convention.
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}

		flags = append(flags, arg)
		// A non-boolean flag written as "--name value" consumes the next arg.
		if !strings.Contains(arg, "=") && takesValue(fs, arg) && i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}

// takesValue reports whether a flag expects a separate value argument.
func takesValue(fs *flag.FlagSet, arg string) bool {
	name := strings.TrimLeft(arg, "-")
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	bf, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !(ok && bf.IsBoolFlag())
}

// Artifact IDs always contain a "/", so a positional argument can never be
// mistaken for a subcommand name.
//
// extractCommand finds the subcommand anywhere in args and returns it along
// with the remaining arguments. Flags may precede it — `deal-kit --kit-dir X`
// with no subcommand at all opens the browser — so the command cannot simply
// be assumed to be args[0].
func extractCommand(args []string) (string, []string) {
	known := commandNames()
	for i, arg := range args {
		if known[arg] {
			rest := make([]string, 0, len(args)-1)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return arg, rest
		}
	}
	return "browse", args
}

// isHelpOrVersion reports a request for help or the version, wherever it
// appears in the arguments.
func isHelpOrVersion(args []string) (help, version bool) {
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "help":
			help = true
		case "-v", "--version", "version":
			version = true
		}
	}
	return help, version
}
