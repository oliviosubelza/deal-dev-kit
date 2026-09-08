package main

import (
	"os"
	"testing"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/kit"
)

// TestParseArgsWiresEveryFlagToEnv is the regression guard for a flag that
// stops reaching its Env field — --dry-run failing to reach env.DryRun, say,
// would silently turn a preview into a real write, and nothing else in the
// suite would notice.
func TestParseArgsWiresEveryFlagToEnv(t *testing.T) {
	_, env, _, flags, err := parseArgs([]string{
		"add", "ui-kit/data-table",
		"--kit-dir", "/local/kit",
		"--repo", "https://example.com/other-kit.git",
		"--ref", "v9.9.9",
		"--offline",
		"--yes",
		"--dry-run",
		"--no-deps",
		"--here",
		"--type", "backend",
	})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if env.KitDir != "/local/kit" {
		t.Errorf("KitDir = %q, want /local/kit", env.KitDir)
	}
	if env.Repo != "https://example.com/other-kit.git" {
		t.Errorf("Repo = %q", env.Repo)
	}
	if env.Ref != "v9.9.9" {
		t.Errorf("Ref = %q", env.Ref)
	}
	if !env.Offline {
		t.Error("Offline = false, want true")
	}
	if !env.AssumeYes {
		t.Error("AssumeYes = false, want true")
	}
	if !env.DryRun {
		t.Error("DryRun = false, want true: a preview flag that does not reach Env can turn into a real write")
	}
	if !env.NoDeps {
		t.Error("NoDeps = false, want true")
	}
	if !env.Here {
		t.Error("Here = false, want true")
	}
	if flags.typeOverride != "backend" {
		t.Errorf("typeOverride = %q, want backend", flags.typeOverride)
	}
}

// TestParseArgsDefaultsLeaveEveryFlagFalse is the other half: absence must
// not accidentally read as presence.
func TestParseArgsDefaultsLeaveEveryFlagFalse(t *testing.T) {
	_, env, _, flags, err := parseArgs([]string{"status"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if env.Offline || env.AssumeYes || env.DryRun || env.NoDeps || env.Here {
		t.Errorf("a bool flag defaulted to true: %+v", env)
	}
	if env.KitDir != "" || env.Ref != "" {
		t.Errorf("a string flag defaulted to non-empty: KitDir=%q Ref=%q", env.KitDir, env.Ref)
	}
	if env.Repo != kit.DefaultRepo {
		t.Errorf("Repo = %q, want the default %q", env.Repo, kit.DefaultRepo)
	}
	if flags.typeOverride != "" || flags.check {
		t.Errorf("commandFlags defaulted to non-zero: %+v", flags)
	}
}

// TestParseArgsAcceptsFlagsAfterThePositional exercises permute: a user who
// writes `deal-kit add ui-kit/data-table --dry-run` must not have --dry-run
// silently ignored just because it came after the artifact id.
func TestParseArgsAcceptsFlagsAfterThePositional(t *testing.T) {
	cmd, env, fs, _, err := parseArgs([]string{"add", "ui-kit/data-table", "--dry-run", "--yes"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if cmd.name != "add" {
		t.Errorf("command = %q, want add", cmd.name)
	}
	if !env.DryRun || !env.AssumeYes {
		t.Errorf("a flag written after the positional was dropped: DryRun=%v AssumeYes=%v", env.DryRun, env.AssumeYes)
	}
	if got := fs.Args(); len(got) != 1 || got[0] != "ui-kit/data-table" {
		t.Errorf("positional args = %v, want [ui-kit/data-table]", got)
	}
}

// TestParseArgsFallsBackToEnvironmentVariables covers the three flags whose
// default comes from an environment variable rather than a bare "".
func TestParseArgsFallsBackToEnvironmentVariables(t *testing.T) {
	t.Setenv("DEAL_KIT_DIR", "/env/kit")
	t.Setenv("DEAL_KIT_REPO", "https://example.com/env-repo.git")
	t.Setenv("DEAL_KIT_REF", "kit-v3.0.0")
	t.Setenv("DEAL_KIT_RELEASE_REPO", "example/deal-kit-releases")

	_, env, _, _, err := parseArgs([]string{"status"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if env.KitDir != "/env/kit" {
		t.Errorf("KitDir = %q, want the DEAL_KIT_DIR fallback", env.KitDir)
	}
	if env.Repo != "https://example.com/env-repo.git" {
		t.Errorf("Repo = %q, want the DEAL_KIT_REPO fallback", env.Repo)
	}
	if env.Ref != "kit-v3.0.0" {
		t.Errorf("Ref = %q, want the DEAL_KIT_REF fallback", env.Ref)
	}
	if env.ReleaseRepo != "example/deal-kit-releases" {
		t.Errorf("ReleaseRepo = %q, want the DEAL_KIT_RELEASE_REPO fallback", env.ReleaseRepo)
	}

	// An explicit flag still wins over the environment.
	_, env2, _, _, err := parseArgs([]string{"status", "--kit-dir", "/flag/kit"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if env2.KitDir != "/flag/kit" {
		t.Errorf("KitDir = %q, want the explicit flag to win over DEAL_KIT_DIR", env2.KitDir)
	}
}

// TestParseArgsFallsBackToBrowseForAnUnrecognisedToken pins what
// extractCommand actually does with a token that names no known command: it
// is not rejected, it is treated as a positional argument to "browse" (the
// same thing a bare `deal-kit` with no subcommand does). The
// "comando desconocido" error path in parseArgs is consequently unreachable
// through this entry point today — lookup always succeeds because
// extractCommand only ever returns a name it already found in commandNames(),
// or the literal fallback "browse", which is itself always a registered
// command. Worth knowing rather than silently relying on a branch that never
// runs; not fixed here, out of scope for this pass.
func TestParseArgsFallsBackToBrowseForAnUnrecognisedToken(t *testing.T) {
	cmd, _, fs, _, err := parseArgs([]string{"not-a-real-command"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if cmd.name != "browse" {
		t.Errorf("command = %q, want browse", cmd.name)
	}
	if got := fs.Args(); len(got) != 1 || got[0] != "not-a-real-command" {
		t.Errorf("positional args = %v, want [not-a-real-command]", got)
	}
}

// TestParseArgsUsesTheRealWorkingDirectory pins that Env.Cwd is not left
// empty or stale.
func TestParseArgsUsesTheRealWorkingDirectory(t *testing.T) {
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	_, env, _, _, parseErr := parseArgs([]string{"status"})
	if parseErr != nil {
		t.Fatalf("parseArgs() error = %v", parseErr)
	}
	if env.Cwd != want {
		t.Errorf("Cwd = %q, want %q", env.Cwd, want)
	}
}
