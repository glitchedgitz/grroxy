package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SetFileInput validates paths before it ever reaches CDP, because
// DOM.setFileInputFiles accepts a path that does not exist without an error
// and the page then "uploads" a zero-byte file — a silent failure that looks
// exactly like success. These cases must fail before GetChromeRemote, so the
// bogus proxy ID below is never reached.
func TestSetFileInputValidatesBeforeCDP(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(real, []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name          string
		selector      string
		clickSelector string
		files         []string
		want          string
	}{
		{"no selector at all", "", "", []string{real}, "either selector or clickSelector"},
		{"both selectors", "#a", "#b", []string{real}, "not both"},
		{"no files", "#a", "", nil, "at least one file"},
		{"empty path", "#a", "", []string{""}, "is empty"},
		{"relative path", "#a", "", []string{"videos/clip.mp4"}, "must be an absolute path"},
		{"missing file", "#a", "", []string{filepath.Join(dir, "nope.mp4")}, "not readable"},
		{"directory", "#a", "", []string{dir}, "is a directory"},
		{"one bad among good", "#a", "", []string{real, filepath.Join(dir, "nope.mp4")}, "files[1]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ProxyMgr.SetFileInput("no-such-proxy", tc.selector, tc.clickSelector, tc.files, 0)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}

	// A fully valid request must get past validation and fail only on the
	// proxy lookup — proving validation isn't rejecting good input.
	err := ProxyMgr.SetFileInput("no-such-proxy", "#a", "", []string{real}, 0)
	if err == nil {
		t.Fatal("expected a proxy lookup error")
	}
	if strings.Contains(err.Error(), "absolute") || strings.Contains(err.Error(), "readable") {
		t.Fatalf("valid input was rejected by validation: %v", err)
	}
	t.Logf("valid input reached the proxy lookup, as intended: %v", err)
}
