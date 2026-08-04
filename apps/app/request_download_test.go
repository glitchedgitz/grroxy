package app

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glitchedgitz/grroxy/internal/config"
)

// RowTargets is embedded rather than named, so "ids" has to promote up to the
// top level of the body. A stray json tag on the embed would nest it instead
// and every request would silently arrive with no targets.
func TestRowTargetsBindsFromTopLevelJSON(t *testing.T) {
	var download DownloadRequest
	if err := json.Unmarshal([]byte(`{"ids":["5"],"part":"both"}`), &download); err != nil {
		t.Fatalf("DownloadRequest: %v", err)
	}
	if len(download.IDs) != 1 || download.IDs[0] != "5" {
		t.Errorf("DownloadRequest ids = %v, want [5]", download.IDs)
	}
	if download.Part != "both" {
		t.Errorf("DownloadRequest part = %q, want both", download.Part)
	}

	var extract ExtractValuesRequest
	if err := json.Unmarshal([]byte(`{"ids":["5"],"name":"Emails"}`), &extract); err != nil {
		t.Fatalf("ExtractValuesRequest: %v", err)
	}
	if len(extract.IDs) != 1 || extract.IDs[0] != "5" {
		t.Errorf("ExtractValuesRequest ids = %v, want [5]", extract.IDs)
	}
	if extract.Name != "Emails" {
		t.Errorf("ExtractValuesRequest name = %q, want Emails", extract.Name)
	}
}

// Targets are ids, not indexes. A lookup by index = 5 returns 5, 5.1, 5.11 and
// so on, so asking for "5" used to hand back every row sharing that index, and
// there was no way to ask for 5.11 on its own.
func TestResolveExtractTargets(t *testing.T) {
	backend := &Backend{}

	ids, skipped, err := backend.resolveExtractTargets(RowTargets{
		IDs: []string{"5", "5.11", "______________5", " 9789 ", "5"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skipped) != 0 {
		t.Errorf("nothing should have been skipped, got %v", skipped)
	}

	// bare ids get padded, an already padded one is taken as is, duplicates
	// collapse, and "5" does not pull in "5.11"
	want := []string{"______________5", "___________5.11", "___________9789"}
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], want[i])
		}
	}
}

func TestResolveExtractTargetsRejects(t *testing.T) {
	backend := &Backend{}

	// longer than the 15 ids are padded to
	if _, skipped, err := backend.resolveExtractTargets(RowTargets{IDs: []string{"1234567890123456"}}); err == nil {
		t.Error("expected an error when no id was usable")
	} else if len(skipped) != 1 {
		t.Errorf("expected the bad id in skipped, got %v", skipped)
	}

	if _, _, err := backend.resolveExtractTargets(RowTargets{}); err == nil {
		t.Error("expected an error when ids is empty")
	}
}

// downloadRequest turns an id into a file name, so an id carrying a separator
// would write outside the requests folder. "../../../etc/pw" is exactly the 15
// chars an id is padded to and has no underscore, so it would otherwise pass
// through untouched.
func TestResolveExtractTargetsRejectsPathSeparators(t *testing.T) {
	backend := &Backend{}

	for _, id := range []string{"../../../etc/pw", `..\..\..\etc\pw`, "a/b", `a\b`} {
		ids, _, err := backend.resolveExtractTargets(RowTargets{IDs: []string{id}})
		if err == nil {
			t.Errorf("%q was accepted, got ids %v", id, ids)
		}
	}
}

// Downloads land in the project working directory — the one the frontend file
// explorer browses — never the process cwd, which is wherever the binary
// happened to be started from.
func TestConfigCWD(t *testing.T) {
	cfg := &config.Config{ProjectsDirectory: "/projects", ProjectID: "proj1"}

	if got, want := cfg.CWD(), filepath.Join("/projects", "proj1"); got != want {
		t.Errorf("CWD() = %q, want %q", got, want)
	}
}
