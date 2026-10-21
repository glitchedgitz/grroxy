package types

import "strings"

// StaticExtTerms are what the "No Media/CSS" filter hides: css, images, fonts,
// video and audio. A request is static when its extension contains one of
// them, the way the filter's old `req.ext ~ '<term>'` clauses matched, so
// ".scss" and ".svgz" count too. Stored on _data as is_static.
var StaticExtTerms = []string{
	"css",
	"jpg", "jpeg", "gif", "png", "webp", "ico", "svg",
	"ttf", "otf", "woff", "woff2", "eot", "dfont", "pfa", "pfb", "fon",
	"mp4", "avi", "mkv", "mov", "webm", "mp3", "wav",
}

// IsStaticExt reports whether a request extension (".css", or "" for none) is
// a static asset. Case-insensitive, like SQLite's LIKE.
func IsStaticExt(ext string) bool {
	ext = strings.ToLower(ext)
	for _, term := range StaticExtTerms {
		if strings.Contains(ext, term) {
			return true
		}
	}
	return false
}
