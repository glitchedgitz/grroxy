package actions

import "github.com/glitchedgitz/grroxy-db/types"

type ModifierReplace struct {
	Key     string `yaml:"key"`
	Search  string `yaml:"search"`
	Replace string `yaml:"replace"`
	Regex   bool   `yaml:"regex"`
}

type Modifier struct {
	Req     types.RequestData `yaml:"req"`
	Replace []ModifierReplace `yaml:"replace"`
	Delete  []string          `yaml:"delete"`
}
