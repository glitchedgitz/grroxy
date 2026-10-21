package types

import "testing"

func TestIsStaticExt(t *testing.T) {
	cases := map[string]bool{
		".css":   true,
		".scss":  true, // substring, as the old filter matched
		".PNG":   true,
		".woff2": true,
		".svgz":  true,
		".mp4":   true,
		"":       false,
		".js":    false,
		".json":  false,
		".html":  false,
		".image": false, // not an extension the filter ever listed
	}
	for ext, want := range cases {
		if got := IsStaticExt(ext); got != want {
			t.Errorf("IsStaticExt(%q) = %v, want %v", ext, got, want)
		}
	}
}
