package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetInvalidURLsWithPlainLinks(t *testing.T) {
	p := Plugin{
		configuration: &configuration{
			RejectPlainLinks:             true,
			AllowedProtocolListLink:      "http,https,mailto",
			AllowedProtocolListPlainText: "http,https,mailto",
		},
	}
	p.plainLinkRegex = regexp.MustCompile(PlainLinkRegexString)
	p.embeddedLinkRegex = regexp.MustCompile(EmbeddedLinkRegexString)
	p.allowedProtocolsRegexLink = regexp.MustCompile(wordListToRegex(p.getConfiguration().AllowedProtocolListLink))
	p.allowedProtocolsRegexPlainText = regexp.MustCompile(wordListToRegex(p.getConfiguration().AllowedProtocolListPlainText))

	var tests = []struct {
		in           *model.Post
		expectedURLs []string
	}{
		{
			in: &model.Post{
				Message: "[test](https://www.github.com)",
			},
			expectedURLs: []string{},
		},
		{
			in: &model.Post{
				Message: "[test](s3://www.github.com)",
			},
			expectedURLs: []string{"s3"},
		},
		{
			in: &model.Post{
				Message: "[test](s3://www.github.com) [test](s4://www.github.com) [test](s3://www.github.com)",
			},
			expectedURLs: []string{"s3", "s4"},
		},
		{
			in: &model.Post{
				Message: "s3://www.github.com [test](s4://www.github.com)",
			},
			expectedURLs: []string{"s3", "s4"},
		},
		{
			in: &model.Post{
				Message: "https://www.github.com [test](s3://www.github.com)",
			},
			expectedURLs: []string{"s3"},
		},
		// Check that disallowed schemes are detected in both embedded and plain links
		{
			in: &model.Post{
				Message: "tel://999999999",
			},
			expectedURLs: []string{"tel"},
		},
		{
			in: &model.Post{
				Message: "tel:999999999",
			},
			expectedURLs: []string{"tel"},
		},
		{
			in: &model.Post{
				Message: "[+999999999](tel:+999999999)",
			},
			expectedURLs: []string{"tel"},
		},
		// Check that allowed schemes are not detected with and without double slashes
		{
			in: &model.Post{
				Message: "[plugin@example.com](mailto:plugin@example.com)",
			},
			expectedURLs: []string{},
		},
		{
			in: &model.Post{
				Message: "[plugin@example.com](mailto://plugin@example.com)",
			},
			expectedURLs: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.in.Message, func(t *testing.T) {
			invalidURLs := p.getInvalidURLs(test.in)
			assert.ElementsMatch(t, test.expectedURLs, invalidURLs)
		})
	}
}

func TestGetInvalidURLsWithoutPlainLinks(t *testing.T) {
	p := Plugin{
		configuration: &configuration{
			RejectPlainLinks:             false,
			AllowedProtocolListLink:      "http,https,mailto",
			AllowedProtocolListPlainText: "http,https,mailto",
		},
	}
	p.plainLinkRegex = regexp.MustCompile(PlainLinkRegexString)
	p.embeddedLinkRegex = regexp.MustCompile(EmbeddedLinkRegexString)
	p.allowedProtocolsRegexLink = regexp.MustCompile(wordListToRegex(p.getConfiguration().AllowedProtocolListLink))
	p.allowedProtocolsRegexPlainText = regexp.MustCompile(wordListToRegex(p.getConfiguration().AllowedProtocolListPlainText))

	var tests = []struct {
		in           *model.Post
		expectedURLs []string
	}{
		{
			in: &model.Post{
				Message: "tel://999999999",
			},
			expectedURLs: []string{},
		},
		{
			in: &model.Post{
				Message: "tel:999999999",
			},
			expectedURLs: []string{},
		},
		{
			in: &model.Post{
				Message: "[+999999999](tel:+999999999)",
			},
			expectedURLs: []string{"tel"},
		},
		{
			in: &model.Post{
				Message: "[plugin@example.com](mailto:plugin@example.com)",
			},
			expectedURLs: []string{},
		},
		{
			in: &model.Post{
				Message: "[plugin@example.com](mailto://plugin@example.com)",
			},
			expectedURLs: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.in.Message, func(t *testing.T) {
			invalidURLs := p.getInvalidURLs(test.in)
			assert.ElementsMatch(t, test.expectedURLs, invalidURLs)
		})
	}
}
