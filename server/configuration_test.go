package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWordListToRegex(t *testing.T) {
	schemes := "https,http,mailto"
	schemesWithSpaces := "https, http, mailto"

	p := newTestPlugin(t, true, schemes, schemes, "")

	t.Run("Build Regex", func(t *testing.T) {
		regexStr := wordListToRegex(p.getConfiguration().AllowedProtocolListLink)

		assert.Equal(t, regexStr, `(?mi)\b(https|http|mailto)\b`)
	})

	p2 := newTestPlugin(t, true, schemesWithSpaces, schemesWithSpaces, "")

	t.Run("Build Regex with extra space", func(t *testing.T) {
		regexStr := wordListToRegex(p2.getConfiguration().AllowedProtocolListLink)

		assert.Equal(t, regexStr, `(?mi)\b(https|http|mailto)\b`)
	})
}
