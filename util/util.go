// Package util is a general utility package for ntfybot
package util

import (
	"os"
	"strings"
)

// FileExists returns true if a file with the given filename exists
func FileExists(filenames ...string) bool {
	for _, filename := range filenames {
		if _, err := os.Stat(filename); err != nil {
			return false
		}
	}
	return true
}

func RemoveString(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func ShortURL(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "http://"), "https://")
}
