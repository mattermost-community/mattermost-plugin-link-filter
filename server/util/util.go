package util

import (
	"strings"
)

func TrimString(textList []string) []string {
	var r []string
	for _, str := range textList {
		str = strings.TrimSpace(str)
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}
