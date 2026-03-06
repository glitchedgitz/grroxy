package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const releasesURL = "https://api.github.com/repos/glitchedgitz/grroxy/releases/latest"

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"` // API URL for downloading (works with private repos)
}

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// CheckLatestVersion fetches the latest release from GitHub.
// Pass an empty token for public repos.
func CheckLatestVersion(token string) (*Release, error) {
	req, err := http.NewRequest("GET", releasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release: %w", err)
	}
	return &release, nil
}

// NeedsUpdate compares current version against the latest release tag.
// Both may optionally have a "v" prefix.
func NeedsUpdate(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")
	return current != latest
}

// FindAsset locates the correct asset for the given binary name and current OS/arch.
func FindAsset(release *Release, binaryName string) (*Asset, error) {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	expected := fmt.Sprintf("%s-%s-%s%s", binaryName, runtime.GOOS, runtime.GOARCH, suffix)

	for _, a := range release.Assets {
		if a.Name == expected {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("no asset found matching %q in release %s", expected, release.TagName)
}

// FindBinaryPath returns the absolute path of a binary using exec.LookPath.
func FindBinaryPath(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("could not find %q in PATH: %w", name, err)
	}
	return filepath.Abs(p)
}

// UpdateBinary downloads the asset from its API URL and replaces the binary at binaryPath.
// For private repos, the token is required to authenticate the download.
func UpdateBinary(assetAPIURL, binaryPath, token string) error {
	req, err := http.NewRequest("GET", assetAPIURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	// Request raw binary via the GitHub API asset endpoint
	req.Header.Set("Accept", "application/octet-stream")
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmpPath := binaryPath + ".tmp"

	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write binary: %w", err)
	}
	tmpFile.Close()

	if runtime.GOOS == "windows" {
		// On Windows, can't overwrite a running binary directly.
		// Rename current to .old, then rename .tmp to original.
		oldPath := binaryPath + ".old"
		os.Remove(oldPath) // clean up from previous update
		if err := os.Rename(binaryPath, oldPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to rename current binary: %w", err)
		}
		if err := os.Rename(tmpPath, binaryPath); err != nil {
			os.Rename(oldPath, binaryPath) // try to restore
			os.Remove(tmpPath)
			return fmt.Errorf("failed to rename new binary: %w", err)
		}
	} else {
		// On Unix, os.Rename is atomic.
		if err := os.Rename(tmpPath, binaryPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to replace binary: %w", err)
		}
	}

	// Ensure executable permissions on non-Windows.
	if runtime.GOOS != "windows" {
		os.Chmod(binaryPath, 0755)
	}

	return nil
}

// CleanupOldBinaries removes .old files left by previous Windows updates.
func CleanupOldBinaries(binaryPath string) {
	oldPath := binaryPath + ".old"
	os.Remove(oldPath)
}

// GetToken returns a GitHub token from the GITHUB_TOKEN or GH_TOKEN environment variable.
func GetToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}
