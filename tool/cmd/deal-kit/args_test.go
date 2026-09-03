package main

import (
	"flag"
	"strings"
	"testing"
)

func newFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("kit-dir", "", "")
	fs.String("type", "", "")
	fs.Bool("yes", false, "")
	fs.Bool("dry-run", false, "")
	return fs
}

func TestPermuteMovesFlagsBeforePositionals(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "flag after positional",
			args: []string{"ui-kit/data-table", "--kit-dir", "/kit", "--dry-run"},
			want: "--kit-dir /kit --dry-run ui-kit/data-table",
		},
		{
			name: "already in order",
			args: []string{"--kit-dir", "/kit", "ui-kit/base"},
			want: "--kit-dir /kit ui-kit/base",
		},
		{
			name: "equals form needs no value",
			args: []string{"web/ui", "--kit-dir=/kit"},
			want: "--kit-dir=/kit web/ui",
		},
		{
			name: "bool flag does not swallow the next argument",
			args: []string{"--yes", "web/ui", "ui-kit/base"},
			want: "--yes web/ui ui-kit/base",
		},
		{
			name: "several positionals interleaved",
			args: []string{"web/ui", "--yes", "ui-kit/base", "--kit-dir", "/kit"},
			want: "--yes --kit-dir /kit web/ui ui-kit/base",
		},
		{
			name: "double dash ends flag parsing",
			args: []string{"--yes", "--", "--not-a-flag"},
			want: "--yes --not-a-flag",
		},
		{
			name: "single dash is positional",
			args: []string{"-", "--yes"},
			want: "--yes -",
		},
		{
			name: "no arguments",
			args: []string{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(permute(newFlagSet(), tt.args), " ")
			if got != tt.want {
				t.Errorf("permute() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPermutedArgsActuallyParse(t *testing.T) {
	fs := newFlagSet()
	args := []string{"ui-kit/data-table", "--kit-dir", "/kit", "--dry-run"}

	if err := fs.Parse(permute(fs, args)); err != nil {
		t.Fatal(err)
	}
	if got := fs.Lookup("kit-dir").Value.String(); got != "/kit" {
		t.Errorf("kit-dir = %q, want /kit", got)
	}
	if got := fs.Lookup("dry-run").Value.String(); got != "true" {
		t.Errorf("dry-run = %q, want true", got)
	}
	if got := fs.Args(); len(got) != 1 || got[0] != "ui-kit/data-table" {
		t.Errorf("positional args = %v, want [ui-kit/data-table]", got)
	}
}

func TestExtractCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCmd  string
		wantRest string
	}{
		{"command first", []string{"init", "--yes"}, "init", "--yes"},
		{"flags before command", []string{"--kit-dir", "/kit", "status"}, "status", "--kit-dir /kit"},
		{"no command opens the browser", []string{"--kit-dir", "/kit"}, "browse", "--kit-dir /kit"},
		{"no arguments at all", []string{}, "browse", ""},
		{"artifact ids are not commands", []string{"add", "ui-kit/data-table"}, "add", "ui-kit/data-table"},
		{"unknown word stays positional", []string{"--yes", "wat"}, "browse", "--yes wat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, rest := extractCommand(tt.args)
			if cmd != tt.wantCmd {
				t.Errorf("command = %q, want %q", cmd, tt.wantCmd)
			}
			if got := strings.Join(rest, " "); got != tt.wantRest {
				t.Errorf("rest = %q, want %q", got, tt.wantRest)
			}
		})
	}
}

func TestIsHelpOrVersion(t *testing.T) {
	tests := []struct {
		args        []string
		help, versn bool
	}{
		{[]string{"--help"}, true, false},
		{[]string{"init", "--help"}, true, false},
		{[]string{"-v"}, false, true},
		{[]string{"--kit-dir", "/k", "version"}, false, true},
		{[]string{"init", "--yes"}, false, false},
	}
	for _, tt := range tests {
		h, v := isHelpOrVersion(tt.args)
		if h != tt.help || v != tt.versn {
			t.Errorf("isHelpOrVersion(%v) = (%v,%v), want (%v,%v)", tt.args, h, v, tt.help, tt.versn)
		}
	}
}

func TestSelfUpdateIsRecognisedAsACommand(t *testing.T) {
	cmd, rest := extractCommand([]string{"self-update", "--check"})
	if cmd != "self-update" {
		t.Errorf("command = %q, want self-update", cmd)
	}
	if len(rest) != 1 || rest[0] != "--check" {
		t.Errorf("rest = %v, want [--check]", rest)
	}
}
