package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oliviosubelza/deal-dev-kit/tool/internal/selfupdate"
)

// SelfUpdate replaces this binary with the newest published release.
// With check set, it only reports what is available.
func SelfUpdate(e Env, check bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate the running binary: %w", err)
	}
	// Follow symlinks so a linked install is updated in place rather than
	// replacing the link with a binary.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	c := selfupdate.New(e.ReleaseRepo)
	release, err := c.Latest()
	if err != nil {
		return err
	}

	current := e.Version
	if current == "" {
		current = "dev"
	}
	fmt.Fprintf(e.Stdout, "  installed  %s\n  latest     %s\n", current, release.Version)

	// "dev" is a local build, which has no version to compare and should not
	// be silently replaced by a release.
	if current == "dev" && !e.AssumeYes {
		return fmt.Errorf("this is a local build; pass --yes to replace it with %s", release.Version)
	}
	if versionsMatch(current, release.Version) {
		fmt.Fprintln(e.Stdout, "\n  already up to date")
		return nil
	}
	if check {
		fmt.Fprintf(e.Stdout, "\n  run `deal-kit self-update` to install %s\n", release.Version)
		return nil
	}

	fmt.Fprintf(e.Stdout, "\n  downloading %s\n", selfupdate.AssetName())
	binary, err := c.Fetch(release)
	if err != nil {
		return err
	}
	if err := selfupdate.Replace(exe, binary); err != nil {
		return err
	}
	fmt.Fprintf(e.Stdout, "  updated    %s → %s\n  at         %s\n", current, release.Version, exe)
	return nil
}

// versionsMatch compares tolerating the leading "v", since the binary reports
// "0.1.5" while the tag is "v0.1.5".
func versionsMatch(current, tag string) bool {
	return current == tag || "v"+current == tag
}
