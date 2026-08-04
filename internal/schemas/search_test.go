package schemas

import (
	"strings"
	"testing"
)

func TestDefaultSearchesHaveUniqueNames(t *testing.T) {
	seen := make(map[string]bool, len(DefaultSearches))

	for _, search := range DefaultSearches {
		if search.Name == "" {
			t.Error("found a default search with an empty name")
			continue
		}

		// _searches has a unique index on name, a duplicate would fail to seed
		if seen[search.Name] {
			t.Errorf("%s: duplicate name", search.Name)
		}

		seen[search.Name] = true
	}
}

// The patterns are written for the JS engine but Compile hands them to RE2,
// which has no lookaround. A default that stops compiling is a default the
// backend extractor can no longer run.
func TestDefaultSearchesCompile(t *testing.T) {
	for _, search := range DefaultSearches {
		if _, err := search.Pattern.Compile(); err != nil {
			t.Errorf("%s: %v", search.Name, err)
		}
	}
}

func TestSearchPatternCompile(t *testing.T) {
	cases := []struct {
		name    string
		pattern SearchPattern
		text    string
		want    []string
	}{
		{
			name:    "plain search is case insensitive by default",
			pattern: SearchPattern{Search: "admin"},
			text:    "Admin admin ADMIN",
			want:    []string{"Admin", "admin", "ADMIN"},
		},
		{
			name:    "plain search matches regex metacharacters literally",
			pattern: SearchPattern{Search: "a.c"},
			text:    "abc a.c",
			want:    []string{"a.c"},
		},
		{
			name:    "case sensitive",
			pattern: SearchPattern{Search: "admin", CaseSensitive: true},
			text:    "Admin admin",
			want:    []string{"admin"},
		},
		{
			name:    "whole word",
			pattern: SearchPattern{Search: "key", WholeWord: true},
			text:    "key monkey key-value",
			want:    []string{"key", "key"},
		},
		{
			// the alternation has to be grouped before the boundaries are
			// added, otherwise \b would only bind to the outer branches
			name:    "whole word around an alternation",
			pattern: SearchPattern{Search: "key|token", Regexp: true, WholeWord: true},
			text:    "monkey token",
			want:    []string{"token"},
		},
		{
			name:    "regexp",
			pattern: SearchPattern{Search: `[a-z]+@[a-z]+\.com`, Regexp: true},
			text:    "reach me at user@example.com ok",
			want:    []string{"user@example.com"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			re, err := tc.pattern.Compile()
			if err != nil {
				t.Fatalf("Compile() error: %v", err)
			}

			got := re.FindAllString(tc.text, -1)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSearchPatternCompileErrors(t *testing.T) {
	if _, err := (SearchPattern{}).Compile(); err == nil {
		t.Error("expected an error for an empty search")
	}

	// lookahead is valid in JS but not in RE2, it has to fail loudly
	if _, err := (SearchPattern{Search: `foo(?=bar)`, Regexp: true}).Compile(); err == nil {
		t.Error("expected an error for a pattern RE2 cannot compile")
	}
}

// The stored string is handed to new RegExp() by the frontend, so a doubled
// backslash reaches the engine as a literal backslash rather than as an escape.
// That is what happens when a pattern is copied out of a JS template literal,
// where the literal itself would have collapsed the pair first.
func TestDefaultSearchesAreNotDoubleEscaped(t *testing.T) {
	for _, search := range DefaultSearches {
		if !search.Pattern.Regexp {
			continue
		}

		// \\\[ and \\\] are the escaped-backslash-then-bracket sequences Link
		// Finder legitimately needs inside its negated class
		stripped := strings.NewReplacer(`\\\[`, "", `\\\]`, "").Replace(search.Pattern.Search)

		if strings.Contains(stripped, `\\`) {
			t.Errorf("%s: pattern contains a doubled backslash, it was likely copied from a JS template literal without halving the escapes", search.Name)
		}

		if strings.ContainsAny(search.Pattern.Search, "\n\r") {
			t.Errorf("%s: pattern contains a literal newline", search.Name)
		}
	}
}
