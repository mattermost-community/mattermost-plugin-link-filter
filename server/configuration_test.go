package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWordListToRegex(t *testing.T) {
	p := Plugin{
		configuration: &configuration{
			AllowedProtocolListLink: "https,http,mailto",
		},
	}

	t.Run("Build Regex", func(t *testing.T) {
		regexStr := wordListToRegex(p.getConfiguration().AllowedProtocolListLink)

		assert.Equal(t, regexStr, `(?mi)\b(https|http|mailto)\b`)
	})

	p2 := Plugin{
		configuration: &configuration{
			AllowedProtocolListLink: "https, http, mailto",
		},
	}

	t.Run("Build Regex with extra space", func(t *testing.T) {
		regexStr := wordListToRegex(p2.getConfiguration().AllowedProtocolListLink)

		assert.Equal(t, regexStr, `(?mi)\b(https|http|mailto)\b`)
	})
}
