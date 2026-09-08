package engram

// Acquiring the `engram` binary.
//
// The plugin ships an .mcp.json that declares `engram mcp --tools=agent`, and
// every one of its hooks shells out to `engram`. Installing the plugin without
// the binary produces a Claude Code that starts, fails to spawn the MCP server
// and fails every hook — which is exactly the state deal-kit used to leave
// behind. So the binary is part of the plan, and it goes first.
//
// deal-kit installs the build for the environment it is itself running in:
// runtime.GOOS/GOARCH pick the asset, never a probe of where `claude` came
// from. There is no WSL/Windows cross-boundary logic on purpose — a Linux
// engram is useless to a Windows-native Claude Code and the other way round,
// and guessing across that boundary installs a binary that cannot run.

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	// GoBin is the Go toolchain, the preferred way to get the binary.
	GoBin = "go"
	// EngramModule is the package `go install` builds.
	EngramModule = "github.com/Gentleman-Programming/engram/cmd/engram"

	// ReleaseBaseURL is where the pinned release's assets live. The tag is
	// appended, then the file name.
	ReleaseBaseURL = "https://github.com/Gentleman-Programming/engram/releases/download"
	// ChecksumsFile is the sha256sum-format manifest published next to the
	// assets. An asset is never installed without matching a line in it.
	ChecksumsFile = "checksums.txt"
)

// testReleaseBase is where a test binary downloads from unless it opts in.
// Port 0 refuses instantly and without a DNS lookup, and the path says what
// the test forgot to do.
const testReleaseBase = "http://127.0.0.1:0/deal-kit-test-no-network-use-fakeRelease"

// releaseBase is the download root. It is a variable for exactly one reason:
// the hermetic test points it at an httptest server, the same seam Lookup is
// for `claude`. Nothing in production ever assigns it.
//
// Under `go test` it starts at a URL that cannot resolve, and that is
// deliberate. A test that builds a Status without an EngramPath gets a
// StepBinaryDownload whether it meant to or not, and this step is the one
// mutation that does not go through Runner — so no test double intercepts it.
// Before this default, running the suite fetched the real 19.5 MB release and
// renamed it over the developer's own engram binary. Reaching the network is
// now something a test asks for explicitly (fakeRelease.serve), and anything
// else fails loudly naming the fixture it should have used.
var releaseBase = defaultReleaseBase()

func defaultReleaseBase() string {
	if testing.Testing() {
		return testReleaseBase
	}
	return ReleaseBaseURL
}

// httpClient is the client the download uses. Also a seam for the test; the
// request always carries the caller's context, so the install budget and the
// SIGINT wired into it apply to a download exactly as they apply to a clone.
var httpClient = &http.Client{}

// maxAssetBytes caps both the download and the extraction. A release binary is
// a few tens of megabytes; the cap is what stops a replaced asset — or a
// crafted archive — from filling the disk before the checksum is even read.
const maxAssetBytes = 256 << 20

// maxChecksumsBytes caps the manifest. It is one short line per published
// asset — a few hundred bytes — and it is read whole into memory to be
// scanned, so a body that never ends must not be allowed to grow without a
// bound just because it is "only text".
const maxChecksumsBytes = 1 << 20

// progressEvery is how often a running transfer reports itself. Slow enough
// that the log of an install stays readable, fast enough that a line which
// stops advancing reads as a stall rather than as the end of the output. It is
// a variable so the test that proves progress arrives mid-transfer does not
// have to wait for a real interval.
var progressEvery = 2 * time.Second

// GoInstallArgs builds the binary from source at the same tag the plugin is
// pinned to, so the two can never drift.
//
// It is preferred over the prebuilt asset because it is what upstream itself
// recommends on Windows: Defender and ESET flag their unsigned prebuilt
// Windows binaries as Trojan:Script/Wacatac.H!ml, a heuristic false positive
// the maintainer will not fix. It is also a plain command, so it reuses the
// Runner, the sanitized environment and the CommandError that every other step
// already goes through.
func GoInstallArgs() []string {
	return []string{GoBin, "install", EngramModule + "@" + MarketplaceTag}
}

// BinaryName is the file name the binary has on this platform.
func BinaryName() string {
	if runtime.GOOS == "windows" {
		return EngramBin + ".exe"
	}
	return EngramBin
}

// releaseVersion is the tag without its leading v, which is how the release
// assets are named.
func releaseVersion() string { return strings.TrimPrefix(MarketplaceTag, "v") }

// assetName is the release asset for one platform, or false when the release
// publishes none. Only the architectures actually shipped are named: a guessed
// URL for anything else would 404 halfway through an install instead of saying
// up front that this machine needs `go`.
func assetName(goos, goarch string) (string, bool) {
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", false
	}
	switch goos {
	case "linux", "darwin":
		return fmt.Sprintf("engram_%s_%s_%s.tar.gz", releaseVersion(), goos, goarch), true
	case "windows":
		return fmt.Sprintf("engram_%s_%s_%s.zip", releaseVersion(), goos, goarch), true
	}
	return "", false
}

// Download is everything a StepBinaryDownload needs. It is resolved when the
// plan is built, so the confirmation screen shows the user the real asset and
// the real destination before anything is fetched.
type Download struct {
	Asset     string // the release asset's file name
	URL       string // where the asset is fetched from
	Checksums string // where its published SHA-256 is fetched from
	Dir       string // the directory the binary is written into
	Name      string // engram, or engram.exe on Windows
	OnPath    bool   // whether Dir is already on PATH
	// Replaces reports that a file is already sitting at Target(). The install
	// renames over it, and Detect's PATH lookup does not see it when Dir is
	// not on PATH — so without saying so out loud, `y` would consent to
	// overwriting something the user never heard about. deal-kit does not keep
	// a backup: it says what it is about to do and lets the user decide.
	Replaces bool
}

// Target is the full path the binary is written to.
func (d Download) Target() string { return filepath.Join(d.Dir, d.Name) }

// binaryStep is how this machine would acquire the binary, or the reason it
// cannot. An empty Step with an empty reason never happens: one of the two is
// always set.
func binaryStep(goos, goarch string, st Status) (Step, string) {
	if st.GoFound() {
		return Step{Kind: StepGoInstall, Args: GoInstallArgs()}, ""
	}
	asset, ok := assetName(goos, goarch)
	if !ok {
		return Step{}, fmt.Sprintf(
			"no hay un binario engram publicado para %s/%s y `go` no está en el PATH: instalarlo a mano",
			goos, goarch)
	}
	dir, onPath, err := binaryDir()
	if err != nil {
		return Step{}, "no se pudo determinar dónde instalar el binario engram: " + err.Error()
	}
	d := Download{
		Asset:     asset,
		URL:       releaseBase + "/" + MarketplaceTag + "/" + asset,
		Checksums: releaseBase + "/" + MarketplaceTag + "/" + ChecksumsFile,
		Dir:       dir,
		Name:      BinaryName(),
		OnPath:    onPath,
	}
	// Resolved while the plan is built, not while it runs: the confirmation
	// screen is the consent gate, and it can only be informed consent if the
	// screen already knows a file is about to be replaced.
	if fi, err := os.Stat(d.Target()); err == nil && fi.Mode().IsRegular() {
		d.Replaces = true
	}
	return Step{
		Kind: StepBinaryDownload,
		Get:  &d,
		Note: downloadNote(d),
	}, ""
}

// downloadNote is the confirmation line for a download step. It names the
// asset, the exact path it lands on, and — when there is one — the file that
// path already holds.
func downloadNote(d Download) string {
	note := "descargar " + d.Asset + " → " + d.Target()
	if d.Replaces {
		note += " (reemplaza el archivo que ya está ahí)"
	}
	return note
}

// binaryCandidates are the directories the binary may be written to, in
// preference order. There is one per platform today; the list exists because
// the rule is "the first candidate that already exists and is on PATH",
// which needs somewhere to express a second candidate when one appears.
func binaryCandidates() ([]string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return nil, errors.New("LOCALAPPDATA no está definida")
		}
		return []string{filepath.Join(base, "Programs", "engram")}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return []string{filepath.Join(home, ".local", "bin")}, nil
}

// binaryDir picks the destination and reports whether it is already on PATH.
//
// deal-kit never edits PATH. That is the same class of global mutation this
// package already refuses when it finds a foreign marketplace named engram:
// the user's shell configuration is theirs. When the destination is not on
// PATH the caller says so, names the directory, and says Claude Code has to be
// restarted — which is the whole of what deal-kit can honestly do.
func binaryDir() (string, bool, error) {
	cands, err := binaryCandidates()
	if err != nil {
		return "", false, err
	}
	if len(cands) == 0 {
		return "", false, errors.New("no hay un directorio de destino para este sistema")
	}
	for _, d := range cands {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() && onPATH(d) {
			return d, true, nil
		}
	}
	return cands[0], onPATH(cands[0]), nil
}

func onPATH(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == "" {
			continue
		}
		if samePath(p, dir) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// fetchBinary downloads the pinned asset, refuses it unless its SHA-256
// matches the published checksums file, extracts the single engram entry and
// moves it into place.
//
// Nothing is written to the destination until the checksum has matched, and
// the binary itself lands through a temporary file in the destination
// directory plus a rename: an interrupted download must never leave a
// truncated executable sitting on someone's PATH.
func fetchBinary(ctx context.Context, d Download, live io.Writer) error {
	if live == nil {
		live = io.Discard
	}
	fmt.Fprintf(live, "descargando %s\n", d.URL)

	want, err := expectedSum(ctx, d)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "deal-kit-engram-")
	if err != nil {
		return fmt.Errorf("no se pudo crear un directorio temporal: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archive := filepath.Join(tmpDir, d.Asset)
	sum, err := downloadTo(ctx, d.URL, archive, live)
	if err != nil {
		return err
	}
	if sum != want {
		// Deliberately before anything touches the destination: a mismatched
		// asset is not installed and then reported, it is never installed.
		return fmt.Errorf("el checksum de %s no coincide con %s (esperado %s, obtenido %s): no se instaló nada",
			d.Asset, ChecksumsFile, want, sum)
	}
	fmt.Fprintf(live, "checksum verificado, instalando en %s\n", d.Target())

	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		return fmt.Errorf("no se pudo crear %s: %w", d.Dir, err)
	}
	return extractBinary(archive, d)
}

// expectedSum reads the published checksums file and returns the hash for this
// asset. A missing line is a refusal, never a skipped check.
func expectedSum(ctx context.Context, d Download) (string, error) {
	resp, err := get(ctx, d.Checksums)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumsBytes))
	if err != nil {
		return "", fmt.Errorf("no se pudo leer %s: %w", d.Checksums, err)
	}
	// Standard sha256sum format: "<hex>  <filename>".
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == d.Asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s no aparece en %s: no se instaló nada", d.Asset, d.Checksums)
}

// downloadTo streams the asset to dst and returns its SHA-256, computed on the
// bytes that were actually written rather than on a second read of the file.
//
// live receives a progress line every progressEvery while the transfer runs.
// The asset is ~20 MB and nothing else prints until the hash is checked, so
// without this the terminal is silent for the whole download — the same
// symptom, on the same screen, that RunStream already fixed for the clone
// `marketplace add` performs.
func downloadTo(ctx context.Context, url, dst string, live io.Writer) (string, error) {
	resp, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("no se pudo escribir la descarga: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	// ContentLength is -1 when the server sends no Content-Length; the
	// progress line then reports bytes without a total rather than inventing
	// a percentage of an unknown size.
	prog := newProgress(live, resp.ContentLength)
	n, err := io.Copy(io.MultiWriter(f, h, prog), io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		return "", fmt.Errorf("se cortó la descarga de %s: %w", url, err)
	}
	if n > maxAssetBytes {
		return "", fmt.Errorf("%s supera el tamaño máximo aceptado", url)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("no se pudo escribir la descarga: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// progress reports a running transfer to the user's terminal. It is a plain
// io.Writer so it can sit in the same MultiWriter as the file and the hash:
// the bytes it counts are exactly the bytes that were written.
//
// Lines are whole lines terminated with \n rather than a carriage-return
// redraw, because live is shared with the commands' own output and is just as
// likely to be a pipe or a log as a terminal.
type progress struct {
	w     io.Writer
	total int64 // Content-Length, or -1 when the server sent none
	n     int64
	last  time.Time
}

func newProgress(w io.Writer, total int64) *progress {
	if w == nil {
		w = io.Discard
	}
	// last starts now, so the first line appears one interval in rather than
	// on the first packet: a fast download stays a single "checksum verificado".
	return &progress{w: w, total: total, last: time.Now()}
}

func (p *progress) Write(b []byte) (int, error) {
	p.n += int64(len(b))
	if time.Since(p.last) >= progressEvery {
		p.last = time.Now()
		fmt.Fprintln(p.w, "  "+p.line())
	}
	return len(b), nil
}

// line is the human-readable state of the transfer. A known total gets a
// percentage; an unknown one gets the byte count alone, never a guess.
func (p *progress) line() string {
	if p.total <= 0 {
		return fmt.Sprintf("%s descargados", humanBytes(p.n))
	}
	return fmt.Sprintf("%s de %s (%d%%)", humanBytes(p.n), humanBytes(p.total), p.n*100/p.total)
}

// humanBytes is the size as a person reads it. Deliberately coarse: this is a
// progress line, not something anything parses.
func humanBytes(n int64) string {
	const unit = 1 << 10
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// get performs one GET that honours ctx, so a download is cancelled by the
// same Ctrl+C and expires on the same install budget as every other step. The
// whole response is returned rather than just its body because the transfer
// reports its progress against Content-Length when the server sends one.
func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo descargar %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("no se pudo descargar %s: %s", url, resp.Status)
	}
	return resp, nil
}

// extractBinary pulls the single engram entry out of the archive.
//
// The destination path is built from constants — the chosen directory and
// BinaryName() — and never from an archive entry's name, so an entry called
// ../../../.bashrc has nowhere to go. Entries that try are skipped outright
// as well, so the archive is reported as not containing the binary rather
// than quietly having one of its members ignored.
func extractBinary(archive string, d Download) error {
	if strings.HasSuffix(d.Asset, ".zip") {
		return extractZip(archive, d)
	}
	return extractTarGz(archive, d)
}

func extractTarGz(archive string, d Download) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%s no es un .tar.gz válido: %w", d.Asset, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("no se pudo leer %s: %w", d.Asset, err)
		}
		if h.Typeflag != tar.TypeReg || !isBinaryEntry(h.Name, d.Name) {
			continue
		}
		return writeBinary(d, tr)
	}
	return fmt.Errorf("%s no contiene %s: no se instaló nada", d.Asset, d.Name)
}

func extractZip(archive string, d Download) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("%s no es un .zip válido: %w", d.Asset, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isBinaryEntry(f.Name, d.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("no se pudo leer %s: %w", d.Asset, err)
		}
		defer rc.Close()
		return writeBinary(d, rc)
	}
	return fmt.Errorf("%s no contiene %s: no se instaló nada", d.Asset, d.Name)
}

// isBinaryEntry reports whether an archive member is the binary we want. An
// absolute name, or one that walks up out of the archive, is refused before
// its base name is even looked at: ../../../engram has the right base name and
// is not something to trust.
func isBinaryEntry(name, want string) bool {
	name = strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return false
	}
	if slices.Contains(strings.Split(name, "/"), "..") {
		return false
	}
	return path.Base(path.Clean(name)) == want
}

// writeBinary lands the extracted bytes atomically. The temporary file is in
// the destination directory so the rename stays on one filesystem, and the
// mode is set before the rename so the file is never on PATH without being
// executable.
func writeBinary(d Download, r io.Reader) error {
	tmp, err := os.CreateTemp(d.Dir, "."+d.Name+".*")
	if err != nil {
		return fmt.Errorf("no se pudo escribir en %s: %w", d.Dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeded

	n, err := io.Copy(tmp, io.LimitReader(r, maxAssetBytes+1))
	if err == nil && n > maxAssetBytes {
		err = fmt.Errorf("%s supera el tamaño máximo aceptado", d.Name)
	}
	if err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpName, 0o755); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpName, d.Target()); err != nil {
		return fmt.Errorf("no se pudo instalar %s: %w", d.Target(), err)
	}
	return nil
}
