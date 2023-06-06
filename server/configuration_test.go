package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestPlugin(rejectPlainLinks bool, allowedProtocolList, allowedProtocolListPlainText string) *Plugin {
	p := Plugin{
		configuration: &configuration{
			RejectPlainLinks:             rejectPlainLinks,
			AllowedProtocolListLink:      allowedProtocolList,
			AllowedProtocolListPlainText: allowedProtocolListPlainText,
		},
	}
	p.plainLinkRegex = regexp.MustCompile(PlainLinkRegexString)
	p.embeddedLinkRegex = regexp.MustCompile(EmbeddedLinkRegexString)
	p.allowedProtocolsRegexLink = regexp.MustCompile(wordListToRegex(p.getConfiguration().AllowedProtocolListLink))
	p.allowedProtocolsRegexPlainText = regexp.MustCompile(wordListToRegex(p.getConfiguration().AllowedProtocolListPlainText))

	return &p
}

func TestWordListToRegex(t *testing.T) {
	schemes := "https,http,mailto"
	schemesWithSpaces := "https, http, mailto"

	p := newTestPlugin(true, schemes, schemes)

	t.Run("Build Regex", func(t *testing.T) {
		regexStr := wordListToRegex(p.getConfiguration().AllowedProtocolListLink)

		assert.Equal(t, regexStr, `(?mi)\b(https|http|mailto)\b`)
	})

	p2 := newTestPlugin(true, schemesWithSpaces, schemesWithSpaces)

	t.Run("Build Regex with extra space", func(t *testing.T) {
		regexStr := wordListToRegex(p2.getConfiguration().AllowedProtocolListLink)

		assert.Equal(t, regexStr, `(?mi)\b(https|http|mailto)\b`)
	})
}
