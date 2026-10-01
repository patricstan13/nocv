// Package util contains the small string helpers shared by higher layers.
package util

import "strings"

// NormalizeName trims surrounding whitespace from a user name.
func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}

// JoinTags renders tags as a comma-separated string.
func JoinTags(tags ...string) string {
	return strings.Join(tags, ",")
}
