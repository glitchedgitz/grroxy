package app

import "testing"

// Names arriving in order, each claimed before the next is resolved — the way
// both proxy creation and the de-dupe migration use NextProxyLabel.
func TestNextProxyLabelNumbersDuplicates(t *testing.T) {
	cases := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			// What a database full of proxies started on a clean slate holds.
			name:  "repeated browser names",
			input: []string{"chrome", "chrome", "chrome", "chrome"},
			want:  []string{"chrome", "chrome 2", "chrome 3", "chrome 4"},
		},
		{
			// An already-numbered name continues the series instead of
			// becoming "chrome 2 2".
			name:  "already numbered duplicates",
			input: []string{"chrome", "chrome", "chrome 2", "chrome 2"},
			want:  []string{"chrome", "chrome 2", "chrome 3", "chrome 4"},
		},
		{
			name:  "distinct names are left alone",
			input: []string{"chrome", "soundon-test", "soundon", "tiktok-artists"},
			want:  []string{"chrome", "soundon-test", "soundon", "tiktok-artists"},
		},
		{
			// Collisions ignore case, but the name keeps the case it was given.
			name:  "case and spacing collide",
			input: []string{"chrome", "Chrome", " chrome "},
			want:  []string{"chrome", "Chrome 2", "chrome 3"},
		},
		{
			// "scan" is never taken, so the year is part of the name.
			name:  "trailing year is not a series number",
			input: []string{"scan 2024", "scan 2024"},
			want:  []string{"scan 2024", "scan 2024 2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			taken := map[string]bool{}
			for i, in := range tc.input {
				got := NextProxyLabel(taken, in)
				if got != tc.want[i] {
					t.Errorf("NextProxyLabel(%q) = %q, want %q", in, got, tc.want[i])
				}
				taken[ProxyLabelKey(got)] = true
			}
		})
	}
}
