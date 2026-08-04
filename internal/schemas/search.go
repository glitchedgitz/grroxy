package schemas

import "github.com/glitchedgitz/pocketbase/models/schema"

var Searches = schema.NewSchema(
	&schema.SchemaField{
		Name:     "name",
		Type:     schema.FieldTypeText,
		Required: true,
	},
	&schema.SchemaField{
		Name: "data",
		Type: schema.FieldTypeJson,
		Options: &schema.JsonOptions{
			MaxSize: 100000,
		},
	},
)

// SearchPattern is the payload stored in the _searches "data" field.
// The frontend reads it as `search` on the store objects, see quicksearch.ts
type SearchPattern struct {
	Search        string `json:"search"`
	Regexp        bool   `json:"regexp"`
	CaseSensitive bool   `json:"caseSensitive"`
	WholeWord     bool   `json:"wholeWord"`
}

// DefaultSearch is a seeded row for the _searches collection
type DefaultSearch struct {
	Name    string
	Pattern SearchPattern
}

// DefaultSearches are seeded into _searches on the launcher.
//
// These patterns are JavaScript dialect. The frontend feeds the stored string
// straight into new RegExp(search, 'gi') (see search_worker.ts), so this is the
// engine that has to accept them. Go only stores and serves them here —
// converting to RE2 is the job of whatever consumes them backend side.
//
// Written as Go raw strings, so a single backslash is a single backslash. The
// old values in quicksearch.ts looked doubled only because they sat inside JS
// template literals, which collapsed `\\s` to `\s` before storing.
var DefaultSearches = []DefaultSearch{
	{
		Name:    "Link Finder",
		Pattern: SearchPattern{Search: `(http(s?)://(www.)?[-a-zA-Z0-9@:%._+~#=]{2,256}.[a-z]{2,6}\b([-a-zA-Z0-9@:%_+.~#?&//=]*))|(?:"|')(((?:[a-zA-Z]{1,10}://|//)[^"'/]{1,}\.[a-zA-Z]{2,}[^"']{0,})|((?:/|\.\./|\./)[^"'><,;|*()(%%$^/\\\[\]][^"'><,;|()]{1,})|([a-zA-Z0-9_\-/]{1,}/[a-zA-Z0-9_\-/]{1,}\.(?:[a-zA-Z]{1,4}|action)(?:[\?|#][^"|']{0,}|))|([a-zA-Z0-9_\-/]{1,}/[a-zA-Z0-9_\-/]{3,}(?:[\?|#][^"|']{0,}|))|([a-zA-Z0-9_\-]{1,}\.(?:php|asp|aspx|jsp|json|action|html|js|txt|xml)(?:[\?|#][^"|']{0,}|)))(?:\s|"|')`, Regexp: true},
	},
	{
		Name:    "Juicy Words",
		Pattern: SearchPattern{Search: `admin|reset|key|api|secret|env|password|username|user|pass|jwt|jenkins|auth|saml|corp|dev|stag|stg|prod|sandbox|uat|test|vpn|cms|secret|private|email|stage|test|devops|staff|internal|lbs`, Regexp: true},
	},
	{
		Name:    "JS Comments",
		Pattern: SearchPattern{Search: `((?:^|\s)\/\/.*$)|(\/\*[^*]*\*+(?:[^/*][^*]*\*+)*\/)`, Regexp: true},
	},
	{
		Name:    "HTML Comments",
		Pattern: SearchPattern{Search: `(<!--[\s\S]*?-->)`, Regexp: true},
	},
	{
		Name:    "Hidden Fields",
		Pattern: SearchPattern{Search: `<(input|textarea)\s+[^>]*type\s*=\s*["']?hidden["']?[^>]*(?:\s+name\s*=\s*["']?([^"'>]+)["']?)?[^>]*(?:\s+value\s*=\s*["']?([^"'>]+)["']?)?[^>]*>`, Regexp: true},
	},
	{
		Name:    "Emails",
		Pattern: SearchPattern{Search: `[A-Za-z0-9\._%+\-]+@[A-Za-z0-9\.\-]+\.[A-Za-z]{2,}`, Regexp: true},
	},
}
