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
