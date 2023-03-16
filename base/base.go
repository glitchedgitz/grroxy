package base

import "strings"

func ParseDatabaseName(site string) string {
	return strings.ReplaceAll(strings.ReplaceAll(site, "://", "__"), ".", "_")
}
