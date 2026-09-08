package engram

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// --- hermetic fixtures ---
//
// Nothing here downloads anything, writes to the real HOME or reads the real
// PATH: the release lives in an httptest server, HOME and LOCALAPPDATA point
// at a t.TempDir, and PATH is emptied of anything that could resolve.

// fakeHome redirects every environment variable the destination is derived
// from and returns the directory the binary would land in.
func fakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	// A PATH with nothing on it, so the destination is never "already on
	// PATH" by accident of the machine running the test.
	t.Setenv("PATH", filepath.Join(home, "no-such-bin"))
	dir, _, err := binaryDir()
	if err != nil {
		t.Fatalf("binaryDir() = %v", err)
	}
	return dir
}

// fakeRelease serves the pinned release from an httptest server and points the
// package at it. sums overrides the published checksums, which is how the
// mismatch is reproduced without corrupting the asset itself.
type fakeRelease struct {
	assets map[string][]byte
	sums   string
	// assetHits, when non-nil, counts requests for anything other than the
	// checksums file — a test uses it to prove fetchBinary skipped the
	// network entirely when a verified local file already sits at Target().
	assetHits *int
}

func (fr fakeRelease) serve(t *testing.T) {
	t.Helper()
	sums := fr.sums
	if sums == "" {
		var b strings.Builder
		for name, body := range fr.assets {
			fmt.Fprintf(&b, "%x  %s\n", sha256.Sum256(body), name)
		}
		sums = b.String()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := path.Base(r.URL.Path)
		if name == ChecksumsFile {
			io.WriteString(w, sums)
			return
		}
		if fr.assetHits != nil {
			*fr.assetHits++
		}
		body, ok := fr.assets[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	prev := releaseBase
	releaseBase = srv.URL
	t.Cleanup(func() { releaseBase = prev })
}

type archiveEntry struct {
	name string
	body []byte
}

// archiveFor packs entries the way the release does: a .tar.gz everywhere and
// a .zip on Windows.
func archiveFor(t *testing.T, asset string, entries ...archiveEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	if strings.HasSuffix(asset, ".zip") {
		zw := zip.NewWriter(&buf)
		for _, e := range entries {
			w, err := zw.Create(e.name)
			if err != nil {
				t.Fatal(err)
			}
			w.Write(e.body)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0o755, Size: int64(len(e.body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		tw.Write(e.body)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// thisAsset is the release asset for the platform the test is running on.
func thisAsset(t *testing.T) string {
	t.Helper()
	a, ok := assetName(runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Skipf("the release publishes no asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return a
}

// readyNoBinary is the machine this whole feature exists for: the plugin is
// installed and enabled, and the binary its MCP server and hooks call is not
// there.
func readyNoBinary() Status {
	return Status{State: StateReady, ClaudePath: "/fake/bin/claude", Version: "1.20.0"}
}

// --- planning ---

func TestAReadyPluginWithoutItsBinaryStillHasAPlan(t *testing.T) {
	// This used to return an empty plan, which is how a machine ended up with
	// the plugin enabled and every hook failing with "engram: not found".
	fakeHome(t)
	p := PlanFor(readyNoBinary())
	if p.Empty() {
		t.Fatalf("a ready plugin with no binary produced no plan (blocked: %q)", p.Blocked())
	}
	if !p.InstallsBinary() {
		t.Errorf("the plan does not install the binary: %v", p.Lines())
	}
}

func TestTheBinaryStepRunsBeforeThePlugin(t *testing.T) {
	// The binary is the engine: installing the plugin first leaves a window
	// where Claude Code has hooks it cannot run.
	fakeHome(t)
	for _, st := range []State{StateMarketplaceMissing, StatePluginMissing, StatePluginDisabled} {
		p := PlanFor(Status{State: st, ClaudePath: "/fake/bin/claude"})
		steps := p.Steps()
		if len(steps) == 0 {
			t.Fatalf("state %v: empty plan (blocked: %q)", st, p.Blocked())
		}
		if k := steps[0].Kind; k != StepBinaryDownload && k != StepGoInstall {
			t.Errorf("state %v: the first step is %v, want the binary:\n%v", st, k, p.Lines())
		}
	}
}

func TestGoInstallIsPreferredWhenTheToolchainIsThere(t *testing.T) {
	// Upstream's own recommendation on Windows, because Defender and ESET
	// flag their unsigned prebuilt binaries as a false positive. It is also a
	// plain command, so it reuses the Runner and CommandError machinery.
	// Windows is forced explicitly (rather than PlanFor's real runtime.GOOS):
	// this preference is Windows-only (see
	// TestTheReleaseAssetIsPreferredOnNonWindowsEvenWithTheToolchain), so the
	// test must not depend on which platform runs the suite.
	fakeHome(t)
	st := readyNoBinary()
	st.GoPath = "/usr/local/go/bin/go"
	steps := planFor("windows", "amd64", st).Steps()
	if len(steps) != 1 {
		t.Fatalf("plan = %v, want a single binary step", planFor("windows", "amd64", st).Lines())
	}
	if steps[0].Kind != StepGoInstall {
		t.Fatalf("step kind = %v, want StepGoInstall", steps[0].Kind)
	}
	want := GoBin + " install " + EngramModule + "@" + MarketplaceTag
	if got := steps[0].Line(); got != want {
		t.Errorf("go install line:\n got %q\nwant %q", got, want)
	}
}

// TestTheReleaseAssetIsPreferredOnNonWindowsEvenWithTheToolchain is the other
// half of the Windows-only preference: on a platform the release actually
// publishes an asset for, the checksum-verified download is cheaper (seconds,
// no compile) and wins even when `go` is on PATH.
func TestTheReleaseAssetIsPreferredOnNonWindowsEvenWithTheToolchain(t *testing.T) {
	dir := fakeHome(t)
	st := readyNoBinary()
	st.GoPath = "/usr/local/go/bin/go"
	for _, goos := range []string{"linux", "darwin"} {
		steps := planFor(goos, "amd64", st).Steps()
		if len(steps) != 1 || steps[0].Kind != StepBinaryDownload {
			t.Errorf("%s: plan = %v, want a single download step", goos, planFor(goos, "amd64", st).Lines())
			continue
		}
		if steps[0].Get == nil || steps[0].Get.Dir != dir {
			t.Errorf("%s: download destination = %v, want %q", goos, steps[0].Get, dir)
		}
	}
}

// TestGoInstallStillCoversAnArchitectureWithNoPublishedAsset preserves the
// pre-existing fallback TestAnUnsupportedPlatformRefusesInsteadOfGuessingAURL
// already covers for an unsupported goarch: restricting the Windows/non-
// Windows preference must not remove `go install` as the escape hatch for a
// platform the release simply never published a binary for.
func TestGoInstallStillCoversAnArchitectureWithNoPublishedAsset(t *testing.T) {
	fakeHome(t)
	st := readyNoBinary()
	st.GoPath = "/usr/local/go/bin/go"
	for _, goos := range []string{"linux", "darwin", "freebsd"} {
		steps := planFor(goos, "riscv64", st).Steps()
		if len(steps) != 1 || steps[0].Kind != StepGoInstall {
			t.Errorf("%s/riscv64: plan = %v, want a single go-install step",
				goos, planFor(goos, "riscv64", st).Lines())
		}
	}
}

func TestTheReleaseAssetIsUsedWhenThereIsNoToolchain(t *testing.T) {
	dir := fakeHome(t)
	steps := PlanFor(readyNoBinary()).Steps()
	if len(steps) != 1 || steps[0].Kind != StepBinaryDownload {
		t.Fatalf("plan = %v, want a single download step", PlanFor(readyNoBinary()).Lines())
	}
	d, ok := PlanFor(readyNoBinary()).Binary()
	if !ok {
		t.Fatal("Binary() reported no download for a plan that has one")
	}
	if d.Dir != dir {
		t.Errorf("destination = %q, want %q", d.Dir, dir)
	}
	if d.OnPath {
		t.Error("the destination was reported as being on PATH, and PATH is empty here")
	}
	if !strings.Contains(steps[0].Line(), d.Asset) {
		t.Errorf("the confirmation line does not name the asset: %q", steps[0].Line())
	}
}

func TestAnUnsupportedPlatformRefusesInsteadOfGuessingAURL(t *testing.T) {
	fakeHome(t)
	p := planFor("plan9", "mips", Status{State: StateMarketplaceMissing, ClaudePath: "/fake/bin/claude"})
	if !p.Empty() {
		t.Fatalf("an unsupported platform produced a plan: %v", p.Lines())
	}
	if !strings.Contains(p.Blocked(), "plan9/mips") {
		t.Errorf("the refusal does not name the platform: %q", p.Blocked())
	}
	if strings.Contains(p.Blocked(), "http") {
		t.Errorf("the refusal carries a guessed URL: %q", p.Blocked())
	}
	// The same platform with a toolchain builds from source instead.
	with := planFor("plan9", "mips", Status{State: StateReady, ClaudePath: "/fake/bin/claude",
		GoPath: "/usr/local/go/bin/go"})
	if with.Empty() {
		t.Errorf("`go` was on PATH and the plan was still refused: %q", with.Blocked())
	}
}

func TestTheAssetNamesAreExactlyWhatTheReleasePublishes(t *testing.T) {
	// Pinned here because a wrong name is a 404 in the middle of an install,
	// and because the assets strip the tag's leading v.
	want := map[string]string{
		"linux/amd64":   "engram_1.20.0_linux_amd64.tar.gz",
		"linux/arm64":   "engram_1.20.0_linux_arm64.tar.gz",
		"darwin/amd64":  "engram_1.20.0_darwin_amd64.tar.gz",
		"darwin/arm64":  "engram_1.20.0_darwin_arm64.tar.gz",
		"windows/amd64": "engram_1.20.0_windows_amd64.zip",
		"windows/arm64": "engram_1.20.0_windows_arm64.zip",
	}
	for k, w := range want {
		parts := strings.SplitN(k, "/", 2)
		got, ok := assetName(parts[0], parts[1])
		if !ok || got != w {
			t.Errorf("%s: asset = %q (%v), want %q", k, got, ok, w)
		}
	}
	// Only what the release actually ships.
	for _, k := range []string{"linux/386", "linux/riscv64", "freebsd/amd64", "windows/386"} {
		parts := strings.SplitN(k, "/", 2)
		if got, ok := assetName(parts[0], parts[1]); ok {
			t.Errorf("%s: invented an asset %q", k, got)
		}
	}
	if got := ReleaseBaseURL + "/" + MarketplaceTag + "/" + want["linux/amd64"]; got !=
		"https://github.com/Gentleman-Programming/engram/releases/download/v1.20.0/engram_1.20.0_linux_amd64.tar.gz" {
		t.Errorf("download URL = %q", got)
	}
}

func TestBothBinaryStepsCountAsADownload(t *testing.T) {
	// --offline has one source of truth, and `go install` fetches modules
	// from the network exactly as the release download fetches an asset.
	fakeHome(t)
	if !PlanFor(readyNoBinary()).NeedsDownload() {
		t.Error("a release download was not reported as needing the network")
	}
	st := readyNoBinary()
	st.GoPath = "/usr/local/go/bin/go"
	if !PlanFor(st).NeedsDownload() {
		t.Error("`go install` was not reported as needing the network")
	}
	// The rule it must not break: enabling what is already on disk is allowed.
	enable := PlanFor(Status{State: StatePluginDisabled, ClaudePath: "/fake/bin/claude",
		EngramPath: "/bin/engram"})
	if enable.NeedsDownload() {
		t.Error("an enable-only plan was reported as needing the network")
	}
}

func TestAnInstallWithoutItsBinaryIsNotVerified(t *testing.T) {
	// Every command succeeded and the plugin reads back as ready, but the
	// engine is missing: the MCP server cannot start and no hook can run.
	ready := Outcome{Status: Status{State: StateReady}}
	if ready.Applied() != true {
		t.Fatal("Applied() = false with no error")
	}
	if ready.Verified() {
		t.Error("Verified() = true although the engram binary is absent")
	}
	ready.Status.EngramPath = "/home/u/.local/bin/engram"
	if !ready.Verified() {
		t.Error("Verified() = false for a ready plugin with its binary in place")
	}
}

func TestTheDownloadStepSurvivesTheDeepCopy(t *testing.T) {
	fakeHome(t)
	p := PlanFor(readyNoBinary())
	steps := p.Steps()
	if steps[0].Get == nil {
		t.Fatal("the download step lost its descriptor in Steps()")
	}
	steps[0].Get.Dir = "/tmp/somewhere-else"
	steps[0].Note = "rm -rf /"
	if d, _ := p.Binary(); d.Dir == "/tmp/somewhere-else" {
		t.Error("rewriting the returned step retargeted the plan's destination")
	}
	if strings.Contains(p.Lines()[0], "rm -rf") {
		t.Errorf("rewriting the returned step changed the plan: %q", p.Lines()[0])
	}
}

// --- fetching ---

func TestADownloadedBinaryLandsInTheDestination(t *testing.T) {
	dir := fakeHome(t)
	asset := thisAsset(t)
	fakeRelease{assets: map[string][]byte{
		asset: archiveFor(t, asset, archiveEntry{BinaryName(), []byte("engram-binary")}),
	}}.serve(t)

	p := PlanFor(readyNoBinary())
	f := &installFake{fakeRunner: newFake(nil), markets: goodMarketplace, plugins: enabledPlugin}
	out := Apply(context.Background(), f, onlyClaude, p, nil)
	if !out.Applied() {
		t.Fatalf("Apply failed: %v", out.Err)
	}
	target := filepath.Join(dir, BinaryName())
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("the binary is not at %s: %v", target, err)
	}
	if string(got) != "engram-binary" {
		t.Errorf("content = %q", got)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm()&0o111 == 0 {
			t.Errorf("mode = %v, want an executable", fi.Mode().Perm())
		}
	}
	// Nothing temporary is left behind: only the binary and the small sidecar
	// that lets a later run recognise it without touching the network again
	// (see TestALocalBinaryVerifiedBySidecarIsNotRedownloaded).
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("the destination holds %v, want the binary plus its sidecar", names)
	}
	if _, err := os.Stat(filepath.Join(dir, sidecarName(BinaryName()))); err != nil {
		t.Errorf("no sidecar checksum file was written: %v", err)
	}
}

// --- verifying a local file before redownloading it ---
//
// The checksum published in checksums.txt covers the release *archive*
// (the .tar.gz/.zip), never the extracted binary that ends up at Target() —
// hashing the local file and comparing it to that published sum can never
// match, they are different byte streams. Instead, deal-kit writes a small
// sidecar next to the binary recording the extracted file's own hash and the
// tag it was fetched for; a later run trusts what is on disk only when both
// still agree, which also means a MarketplaceTag bump correctly forces a
// re-fetch rather than being silently missed.

func TestALocalBinaryVerifiedBySidecarIsNotRedownloaded(t *testing.T) {
	fakeHome(t)
	asset := thisAsset(t)
	content := []byte("engram-binary-content")
	assetHits := 0
	fakeRelease{assets: map[string][]byte{
		asset: archiveFor(t, asset, archiveEntry{BinaryName(), content}),
	}, assetHits: &assetHits}.serve(t)

	d, ok := PlanFor(readyNoBinary()).Binary()
	if !ok {
		t.Fatal("no download step to inspect")
	}
	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.Target(), content, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSidecarForTest(t, d, content, MarketplaceTag)

	var live strings.Builder
	if err := fetchBinary(context.Background(), d, &live); err != nil {
		t.Fatalf("fetchBinary = %v", err)
	}
	if assetHits != 0 {
		t.Errorf("the asset was fetched %d time(s); a verified local file should skip the network entirely", assetHits)
	}
	if !strings.Contains(live.String(), "ya está instalado") {
		t.Errorf("nothing told the user the local file was trusted: %q", live.String())
	}
	got, err := os.ReadFile(d.Target())
	if err != nil || string(got) != string(content) {
		t.Errorf("content = %q (%v), want %q unchanged", got, err, content)
	}
}

func TestALocalFileWithNoSidecarIsRedownloaded(t *testing.T) {
	// The counterpart to the test above: a file deal-kit never wrote a
	// sidecar for (garbage, or an install from before this fix existed) must
	// not be trusted just because something is sitting at Target().
	fakeHome(t)
	asset := thisAsset(t)
	content := []byte("the-real-release-content")
	fakeRelease{assets: map[string][]byte{
		asset: archiveFor(t, asset, archiveEntry{BinaryName(), content}),
	}}.serve(t)

	d, ok := PlanFor(readyNoBinary()).Binary()
	if !ok {
		t.Fatal("no download step to inspect")
	}
	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.Target(), []byte("someone else's file"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := fetchBinary(context.Background(), d, nil); err != nil {
		t.Fatalf("fetchBinary = %v", err)
	}
	got, err := os.ReadFile(d.Target())
	if err != nil || string(got) != string(content) {
		t.Errorf("content = %q (%v), want the real release %q", got, err, content)
	}
}

func TestALocalBinaryFromAnOlderTagIsRedownloaded(t *testing.T) {
	fakeHome(t)
	asset := thisAsset(t)
	content := []byte("engram-binary-content")
	assetHits := 0
	fakeRelease{assets: map[string][]byte{
		asset: archiveFor(t, asset, archiveEntry{BinaryName(), content}),
	}, assetHits: &assetHits}.serve(t)

	d, ok := PlanFor(readyNoBinary()).Binary()
	if !ok {
		t.Fatal("no download step to inspect")
	}
	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.Target(), content, 0o755); err != nil {
		t.Fatal(err)
	}
	// Same bytes, but the sidecar says an older tag: a version bump must not
	// be silently missed just because the old binary happens to still be
	// sitting there.
	writeSidecarForTest(t, d, content, "v1.19.0")

	if err := fetchBinary(context.Background(), d, nil); err != nil {
		t.Fatalf("fetchBinary = %v", err)
	}
	if assetHits == 0 {
		t.Error("a binary from an older tag was trusted without re-fetching")
	}
}

func TestANonExecutableLocalBinaryIsNotTrustedEvenWithAMatchingSidecar(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX executable bit to strip")
	}
	fakeHome(t)
	asset := thisAsset(t)
	content := []byte("engram-binary-content")
	assetHits := 0
	fakeRelease{assets: map[string][]byte{
		asset: archiveFor(t, asset, archiveEntry{BinaryName(), content}),
	}, assetHits: &assetHits}.serve(t)

	d, ok := PlanFor(readyNoBinary()).Binary()
	if !ok {
		t.Fatal("no download step to inspect")
	}
	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Correct content, correct sidecar, but the file lost its executable bit.
	if err := os.WriteFile(d.Target(), content, 0o644); err != nil {
		t.Fatal(err)
	}
	writeSidecarForTest(t, d, content, MarketplaceTag)

	if err := fetchBinary(context.Background(), d, nil); err != nil {
		t.Fatalf("fetchBinary = %v", err)
	}
	if assetHits == 0 {
		t.Error("a non-executable file was trusted without re-fetching")
	}
	fi, err := os.Stat(d.Target())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Error("the re-fetched binary is still not executable")
	}
}

// writeSidecarForTest writes the sidecar file fetchBinary itself would have
// written for content, but pinned to an explicit tag rather than whatever
// MarketplaceTag happens to be — the only way to construct a
// from-an-older-tag fixture.
func writeSidecarForTest(t *testing.T, d Download, content []byte, tag string) {
	t.Helper()
	sum := sha256.Sum256(content)
	path := filepath.Join(d.Dir, sidecarName(d.Name))
	if err := os.WriteFile(path, []byte(hex.EncodeToString(sum[:])+"  "+tag+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAMismatchedChecksumInstallsNothing(t *testing.T) {
	dir := fakeHome(t)
	asset := thisAsset(t)
	fakeRelease{
		assets: map[string][]byte{
			asset: archiveFor(t, asset, archiveEntry{BinaryName(), []byte("tampered")}),
		},
		// A checksums file that does not describe the asset being served is
		// what a swapped release asset looks like from here.
		sums: strings.Repeat("0", 64) + "  " + asset + "\n",
	}.serve(t)

	d, ok := PlanFor(readyNoBinary()).Binary()
	if !ok {
		t.Fatal("no download step to exercise")
	}
	err := fetchBinary(context.Background(), d, nil)
	if err == nil {
		t.Fatal("a mismatched checksum was installed")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("the error does not say what failed: %v", err)
	}
	assertNothingWritten(t, dir)
}

func TestAChecksumThatIsNotPublishedIsRefused(t *testing.T) {
	dir := fakeHome(t)
	asset := thisAsset(t)
	fakeRelease{
		assets: map[string][]byte{asset: archiveFor(t, asset, archiveEntry{BinaryName(), []byte("x")})},
		sums:   "deadbeef  some-other-file.tar.gz\n",
	}.serve(t)

	d, _ := PlanFor(readyNoBinary()).Binary()
	if err := fetchBinary(context.Background(), d, nil); err == nil {
		t.Fatal("an asset with no published checksum was installed")
	}
	assertNothingWritten(t, dir)
}

func TestAnArchiveEntryCannotEscapeTheDestination(t *testing.T) {
	dir := fakeHome(t)
	asset := thisAsset(t)
	// The entry's base name is exactly the binary we are looking for; only
	// its path is hostile. Writing it would land outside the destination.
	escape := "../../../../" + BinaryName()
	fakeRelease{assets: map[string][]byte{
		asset: archiveFor(t, asset, archiveEntry{escape, []byte("pwned")}),
	}}.serve(t)

	d, _ := PlanFor(readyNoBinary()).Binary()
	err := fetchBinary(context.Background(), d, nil)
	if err == nil {
		t.Fatal("a traversing entry was extracted")
	}
	if !strings.Contains(err.Error(), "no contiene") {
		t.Errorf("unexpected error: %v", err)
	}
	assertNothingWritten(t, dir)
	outside := filepath.Clean(filepath.Join(dir, escape))
	if _, err := os.Stat(outside); err == nil {
		t.Fatalf("the entry escaped to %s", outside)
	}
}

func TestAMissingAssetIsReportedRatherThanHalfInstalled(t *testing.T) {
	dir := fakeHome(t)
	asset := thisAsset(t)
	fakeRelease{sums: strings.Repeat("a", 64) + "  " + asset + "\n"}.serve(t)
	d, _ := PlanFor(readyNoBinary()).Binary()
	if err := fetchBinary(context.Background(), d, nil); err == nil {
		t.Fatal("a 404 was treated as a successful download")
	}
	assertNothingWritten(t, dir)
}

// assertNothingWritten checks the destination directory holds no binary. A
// refused download must not leave a truncated or unverified file on PATH.
func assertNothingWritten(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // never created, which is the strongest form of "nothing"
	}
	for _, e := range entries {
		t.Errorf("the destination holds %q after a refused download", e.Name())
	}
}

func TestTheGoInstallStepRunsTheResolvedToolchain(t *testing.T) {
	// Forces Windows (see TestGoInstallIsPreferredWhenTheToolchainIsThere for
	// why): this test is about runStep's StepGoInstall dispatch, not about
	// which platform prefers it, so it must not depend on the host running
	// the suite.
	fakeHome(t)
	st := readyNoBinary()
	st.GoPath = "/fake/bin/go"
	line := strings.Join(GoInstallArgs(), " ")
	f := &installFake{
		fakeRunner: newFake(map[string]reply{line: {}}),
		markets:    goodMarketplace, plugins: enabledPlugin,
	}
	out := Apply(context.Background(), f, found, planFor("windows", "amd64", st), nil)
	if !out.Applied() {
		t.Fatalf("Apply failed: %v", out.Err)
	}
	var ran bool
	for _, c := range f.calls {
		if c == line {
			ran = true
		}
	}
	if !ran {
		t.Errorf("`go install` never ran; calls were %v", f.calls)
	}
}

func TestTheDestinationIsReportedAsOnPathWhenItIs(t *testing.T) {
	// deal-kit never edits PATH, so the only thing it can do is tell the
	// truth about whether the destination is on it.
	dir := fakeHome(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, onPath, err := binaryDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != dir || !onPath {
		t.Errorf("binaryDir() = (%q, %v), want (%q, true)", got, onPath, dir)
	}
}

// --- default-deny ---

func TestAPlanBuiltWithoutAFixtureCannotReachTheRelease(t *testing.T) {
	// The whole suite used to download the real 19.5 MB asset and rename it
	// over the developer's own ~/.local/bin/engram, because any Status with no
	// EngramPath grows a download step and that step bypasses the Runner every
	// other command is faked through. Opting in is now explicit; forgetting is
	// a loud, instant failure.
	d, ok := PlanFor(readyNoBinary()).Binary()
	if !ok {
		t.Fatal("no download step to inspect")
	}
	for _, u := range []string{d.URL, d.Checksums} {
		if !strings.HasPrefix(u, testReleaseBase+"/") {
			t.Fatalf("a test-binary plan points at %q, want the refusing %q", u, testReleaseBase)
		}
	}
	if strings.Contains(d.URL, "github.com") {
		t.Fatalf("a test-binary plan points at the real release: %q", d.URL)
	}

	err := fetchBinary(context.Background(), d, nil)
	if err == nil {
		t.Fatal("the refusing release base served a download")
	}
	if !strings.Contains(err.Error(), "fakeRelease") {
		t.Errorf("the failure does not name the fixture the test should have used: %v", err)
	}
}

func TestTheDestinationNeverLeavesTheScratchHome(t *testing.T) {
	// The other half of the trap: even a download that somehow succeeded must
	// not be able to land on the real PATH. TestMain redirects HOME before any
	// test runs, so this holds without a fakeHome call.
	dir, _, err := binaryDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, home) {
		t.Fatalf("binaryDir() = %q, outside the redirected home %q", dir, home)
	}
	if strings.Contains(dir, os.Getenv("USER")+"/.local/bin") && !strings.Contains(dir, os.TempDir()) {
		t.Fatalf("binaryDir() = %q, which looks like the developer's real bin directory", dir)
	}
}

// --- progress ---

func TestTheDownloadReportsProgressBeforeItFinishes(t *testing.T) {
	// Mirrors TestExecRunnerStreamsBeforeTheCommandExits: the server does not
	// finish the response until the test has seen a progress line, so a buffer
	// flushed at the end cannot pass. Minutes of a static "descargando ..." on
	// a slow link is the silent terminal this package already fixed once for
	// the clone `marketplace add` performs.
	// Zero means "every chunk": the test must not spend a real interval
	// waiting, and what is under test is that the line is emitted while the
	// body is still open, not how often.
	prev := progressEvery
	progressEvery = 0
	t.Cleanup(func() { progressEvery = prev })

	const chunk = 64 << 10
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(chunk*2))
		w.WriteHeader(http.StatusOK)
		w.Write(make([]byte, chunk))
		w.(http.Flusher).Flush()
		<-release
		w.Write(make([]byte, chunk))
	}))
	t.Cleanup(srv.Close)

	seen := make(chan struct{})
	live := &signalOnWrite{fire: seen}
	done := make(chan error, 1)
	dst := filepath.Join(t.TempDir(), "asset")
	go func() {
		_, err := downloadTo(context.Background(), srv.URL+"/asset", dst, live)
		done <- err
	}()

	select {
	case <-seen:
	case <-time.After(10 * time.Second):
		close(release)
		t.Fatal("nothing was written before the transfer finished: the progress is buffered")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got := live.String()
	if !strings.Contains(got, "KB") && !strings.Contains(got, "MB") {
		t.Errorf("the progress never reports how much was transferred:\n%s", got)
	}
	// Content-Length was sent, so the line says what it is a fraction of.
	if !strings.Contains(got, "%") || !strings.Contains(got, " de ") {
		t.Errorf("the progress does not report the total the server announced:\n%s", got)
	}
}

func TestProgressWithoutAContentLengthReportsBytesNotAGuess(t *testing.T) {
	p := newProgress(io.Discard, -1)
	p.n = 3 << 20
	if got := p.line(); strings.Contains(got, "%") {
		t.Errorf("a percentage was invented for an unknown total: %q", got)
	}
	p.total = 6 << 20
	if got := p.line(); !strings.Contains(got, "50%") {
		t.Errorf("line = %q, want half of the announced total", got)
	}
}

// --- replacing an existing file ---

func TestAnExistingBinaryAtTheDestinationIsAnnounced(t *testing.T) {
	// binaryDir falls through to the first candidate when it exists but is not
	// on PATH, and writeBinary renames over whatever sits there. Detect's PATH
	// lookup cannot see that file, so nothing else in the run would mention
	// it, and `y` would be consent to an overwrite the user never heard of.
	dir := fakeHome(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, BinaryName())
	if err := os.WriteFile(target, []byte("someone else's engram"), 0o755); err != nil {
		t.Fatal(err)
	}

	d, ok := PlanFor(readyNoBinary()).Binary()
	if !ok {
		t.Fatal("no download step to inspect")
	}
	if !d.Replaces {
		t.Fatalf("a pre-existing %s was not reported as being replaced", target)
	}
	line := PlanFor(readyNoBinary()).Lines()[0]
	if !strings.Contains(line, "reemplaza") {
		t.Errorf("the confirmation line does not say the file is replaced: %q", line)
	}
	if !strings.Contains(line, target) {
		t.Errorf("the confirmation line does not name the exact path: %q", line)
	}
}

func TestAnEmptyDestinationIsNotReportedAsAReplacement(t *testing.T) {
	// The counterpart: the common case must not grow a warning about a file
	// that is not there.
	fakeHome(t)
	d, _ := PlanFor(readyNoBinary()).Binary()
	if d.Replaces {
		t.Error("an empty destination was reported as holding a file to replace")
	}
	if strings.Contains(PlanFor(readyNoBinary()).Lines()[0], "reemplaza") {
		t.Errorf("the confirmation line warns about nothing: %q", PlanFor(readyNoBinary()).Lines()[0])
	}
}
