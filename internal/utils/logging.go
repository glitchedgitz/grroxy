package utils

import (
	"encoding/json"
	"fmt"
)

// Return printmap
func PrintAnyData(m any) string {
	s := ""
	// convert to json
	b, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		return fmt.Sprint(m)
	}
	s = string(b)
	return s
}
