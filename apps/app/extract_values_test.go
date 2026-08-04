package app

import (
	"regexp"
	"strings"
	"testing"
)

// matchAll is what turns a compiled quick search into the values
// /api/extract/values hands back, so group selection and the limit are the
// parts worth pinning down.

func TestMatchAllWholeMatch(t *testing.T) {
	re := regexp.MustCompile(`[a-z]+@[a-z]+\.com`)

	got := matchAll(re, "a@x.com and b@y.com", 0, 0)

	if strings.Join(got, ",") != "a@x.com,b@y.com" {
		t.Errorf("got %v", got)
	}
}

func TestMatchAllReturnsGroup(t *testing.T) {
	re := regexp.MustCompile(`Set-Cookie: ([^=]+)=([^;]+)`)

	got := matchAll(re, "Set-Cookie: session=abc123; Path=/", 2, 0)

	if len(got) != 1 || got[0] != "abc123" {
		t.Errorf("got %v, want the second group", got)
	}
}

// An out of range group is a caller mistake that should not silently drop every
// match, so it falls back to the whole match.
func TestMatchAllOutOfRangeGroupFallsBack(t *testing.T) {
	re := regexp.MustCompile(`(a)b`)

	got := matchAll(re, "ab", 5, 0)

	if len(got) != 1 || got[0] != "ab" {
		t.Errorf("got %v, want the whole match", got)
	}
}

// A group that did not participate has nothing to extract, so that match is
// skipped rather than reported as an empty value.
func TestMatchAllSkipsUnmatchedGroup(t *testing.T) {
	re := regexp.MustCompile(`x(?:(a)|b)`)

	got := matchAll(re, "xb xa", 1, 0)

	if len(got) != 1 || got[0] != "a" {
		t.Errorf("got %v, want only the branch that matched", got)
	}
}

func TestMatchAllLimit(t *testing.T) {
	re := regexp.MustCompile(`\d`)

	if got := matchAll(re, "12345", 0, 2); len(got) != 2 {
		t.Errorf("got %d matches, want 2", len(got))
	}

	// 0 means every match, not none
	if got := matchAll(re, "12345", 0, 0); len(got) != 5 {
		t.Errorf("got %d matches, want 5", len(got))
	}
}
