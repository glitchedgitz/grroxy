package actions

import "github.com/glitchedgitz/grroxy-db/internal/types"

type ModifierReplace struct {
	Search string `yaml:"search"`
	Value  string `yaml:"value"`
	Regex  bool   `yaml:"regex"`
}

type ModifierSet struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type Modifier struct {
	Req     types.RequestData `yaml:"req"`
	Replace []ModifierReplace `yaml:"replace"`
	Delete  []string          `yaml:"delete"`
}
