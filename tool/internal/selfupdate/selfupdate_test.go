package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGitHub serves a release the way GitHub and GoReleaser do.
func fakeGitHub(t *testing.T, tag string, binary []byte, corruptSums bool) *Client {
	t.Helper()
	sum := sha256.Sum256(binary)
	line := hex.EncodeToString(sum[:])
	if corruptSums {
		line = strings.Repeat("0", 64)
	}
	sums := fmt.Sprintf("%s  %s\n%s  some-other-file\n", line, AssetName(), strings.Repeat("a", 64))

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/kit/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q}`, tag)
	})
	mux.HandleFunc("/acme/kit/releases/download/"+tag+"/"+AssetName(), func(w http.ResponseWriter, _ *http.Request) {
		w.Write(binary)
	})
	mux.HandleFunc("/acme/kit/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, sums)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := New("acme/kit")
	c.BaseURL, c.DownloadURL = srv.URL, srv.URL
	return c
}

func TestLatestReadsTheTag(t *testing.T) {
	c := fakeGitHub(t, "v1.2.3", []byte("binary"), false)
	got, err := c.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "v1.2.3" {
		t.Errorf("version = %q, want v1.2.3", got.Version)
	}
	if !strings.HasSuffix(got.AssetURL, AssetName()) {
		t.Errorf("asset URL = %q, want it to end in %q", got.AssetURL, AssetName())
	}
}

func TestLatestWithNoRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := New("acme/kit")
	c.BaseURL = srv.URL
	_, err := c.Latest()
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "no se encontró ninguna versión publicada") {
		t.Errorf("error = %q, want it to name the cause", err)
	}
}

func TestFetchVerifiesTheChecksum(t *testing.T) {
	want := []byte("a plausible binary")
	c := fakeGitHub(t, "v1.0.0", want, false)

	rel, err := c.Latest()
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Fetch(rel)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("downloaded %q, want %q", got, want)
	}
}

func TestFetchRejectsATamperedBinary(t *testing.T) {
	// A binary that does not match its published checksum must never reach
	// disk: this is the only thing standing between a bad download and code
	// running with the developer's privileges.
	c := fakeGitHub(t, "v1.0.0", []byte("tampered"), true)
	rel, _ := c.Latest()

	_, err := c.Fetch(rel)
	if err == nil {
		t.Fatal("expected the checksum check to reject the download")
	}
	if !strings.Contains(err.Error(), "no coincide (se esperaba") {
		t.Errorf("error = %q", err)
	}
}

func TestChecksumForPicksTheRightLine(t *testing.T) {
	sums := "aaa  other_file\nbbb  " + AssetName() + "\nccc  yet_another\n"
	got, ok := checksumFor(sums, AssetName())
	if !ok || got != "bbb" {
		t.Errorf("checksumFor = (%q, %v), want (bbb, true)", got, ok)
	}
	if _, ok := checksumFor(sums, "missing_asset"); ok {
		t.Error("reported a checksum for an asset that is not listed")
	}
}

func TestReplaceSwapsTheBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deal-kit")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(path, []byte("new")); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil || string(got) != "new" {
		t.Fatalf("content = %q (%v), want \"new\"", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("mode = %v, want the new binary to stay executable", info.Mode())
	}
}

func TestReplaceLeavesNoStagingFilesBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deal-kit")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Replace(path, []byte("new")); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "deal-kit" {
			t.Errorf("left %q behind in the install directory", e.Name())
		}
	}
}

func TestReplaceOnAnUnwritableDirectoryKeepsTheOldBinary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permissions do not apply")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "deal-kit")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if err := Replace(path, []byte("new")); err == nil {
		t.Fatal("expected an error on a read-only directory")
	}
	// The working binary must survive a failed update.
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "old" {
		t.Errorf("content = %q (%v), want the old binary intact", got, err)
	}
}
