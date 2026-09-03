// Package selfupdate replaces the running deal-kit binary with a newer
// published release, so a developer never has to re-run the install script.
package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultRepo is the repository releases are published from.
const DefaultRepo = "oliviosubelza/deal-dev-kit"

// Release is a published version and where to fetch its assets.
type Release struct {
	Version   string
	AssetURL  string
	ChecksURL string
}

// Client fetches releases. BaseURL and DownloadURL exist so tests can point at
// a local server instead of GitHub.
type Client struct {
	Repo        string
	BaseURL     string // GitHub API base, default https://api.github.com
	DownloadURL string // release download base, default https://github.com
	HTTP        *http.Client
}

// New returns a client with sensible defaults.
func New(repo string) *Client {
	if repo == "" {
		repo = DefaultRepo
	}
	return &Client{
		Repo:        repo,
		BaseURL:     "https://api.github.com",
		DownloadURL: "https://github.com",
		HTTP:        &http.Client{Timeout: 60 * time.Second},
	}
}

// AssetName is the published binary for the running platform.
func AssetName() string {
	name := fmt.Sprintf("deal-kit_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Latest reports the newest published release.
func (c *Client) Latest() (Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.BaseURL, c.Repo)
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return Release{}, fmt.Errorf("checking for a newer version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("no published release found for %s", c.Repo)
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("checking for a newer version: %s", resp.Status)
	}

	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("reading the release: %w", err)
	}
	if body.TagName == "" {
		return Release{}, fmt.Errorf("the latest release has no tag")
	}

	base := fmt.Sprintf("%s/%s/releases/download/%s", c.DownloadURL, c.Repo, body.TagName)
	return Release{
		Version:   body.TagName,
		AssetURL:  base + "/" + AssetName(),
		ChecksURL: base + "/checksums.txt",
	}, nil
}

// Fetch downloads the release binary and verifies it against the published
// checksums. An unverified binary is never written anywhere it could be run.
func (c *Client) Fetch(r Release) ([]byte, error) {
	binary, err := c.get(r.AssetURL)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", AssetName(), err)
	}
	sums, err := c.get(r.ChecksURL)
	if err != nil {
		return nil, fmt.Errorf("downloading checksums: %w", err)
	}

	want, ok := checksumFor(string(sums), AssetName())
	if !ok {
		return nil, fmt.Errorf("no checksum published for %s", AssetName())
	}
	sum := sha256.Sum256(binary)
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want) {
		return nil, fmt.Errorf("checksum mismatch for %s (expected %s, got %s)", AssetName(), want, got)
	}
	return binary, nil
}

func (c *Client) get(url string) ([]byte, error) {
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 128<<20))
}

// checksumFor finds a file's hash in a GoReleaser checksums.txt.
func checksumFor(sums, name string) (string, bool) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			return fields[0], true
		}
	}
	return "", false
}

// Replace swaps the binary at path for newBinary.
//
// The current executable is moved aside rather than overwritten: Windows
// refuses to write a running image, but does allow renaming it. The old file
// is then removed on a best-effort basis, since Windows keeps it locked until
// the process exits.
func Replace(path string, newBinary []byte) error {
	dir := filepath.Dir(path)

	// Stage in the same directory so the final move is a rename on the same
	// filesystem, which is atomic, rather than a copy that can half-finish.
	tmp, err := os.CreateTemp(dir, ".deal-kit-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	staged := tmp.Name()
	defer os.Remove(staged)

	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return err
	}

	old := path + ".old"
	os.Remove(old) // a previous update may have left one behind
	if err := os.Rename(path, old); err != nil {
		return fmt.Errorf("cannot move the current binary aside: %w", err)
	}
	if err := os.Rename(staged, path); err != nil {
		// Put the working binary back rather than leaving nothing installed.
		if rollback := os.Rename(old, path); rollback != nil {
			return fmt.Errorf("update failed and the old binary could not be restored from %s: %w", old, err)
		}
		return fmt.Errorf("installing the new binary: %w", err)
	}
	os.Remove(old)
	return nil
}
