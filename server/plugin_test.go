package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetInvalidURLsWithPlainLinks(t *testing.T) {
	p := newTestPlugin(true, "http,https,mailto", "http,https,mailto", "")

	var tests = []struct {
		name         string
		in           *model.Post
		expectedURLs []string
	}{
		{
			name: "allowed embedded links are allowed",
			in: &model.Post{
				Message: "[test](https://www.github.com)",
			},
			expectedURLs: []string{},
		},
		{
			name: "non-allowed embedded links are rejected",
			in: &model.Post{
				Message: "[test](s3://www.github.com)",
			},
			expectedURLs: []string{"s3"},
		},
		{
			name: "non-allowed embedded links are rejected with multiple embedded links",
			in: &model.Post{
				Message: "[test](s3://www.github.com) [test](s4://www.github.com) [test](s3://www.github.com)",
			},
			expectedURLs: []string{"s3", "s4"},
		},
		{
			name: "non-allowed embedded links are rejected with multiple links and plain links",
			in: &model.Post{
				Message: "s3://www.github.com [test](s4://www.github.com)",
			},
			expectedURLs: []string{"s3", "s4"},
		},
		{
			name: "non-allowed embedded links are rejected with multiple links and plain links",
			in: &model.Post{
				Message: "https://www.github.com [test](s3://www.github.com)",
			},
			expectedURLs: []string{"s3"},
		},
		{
			name: "non-allowed plain with double slashes links are rejected",
			in: &model.Post{
				Message: "tel://999999999",
			},
			expectedURLs: []string{"tel"},
		},
		{
			name: "non-allowed plain without double slashes links are rejected",
			in: &model.Post{
				Message: "tel:999999999",
			},
			expectedURLs: []string{"tel"},
		},
		{
			name: "non-allowed embedded links without double slashes are rejected",
			in: &model.Post{
				Message: "[+999999999](tel:+999999999)",
			},
			expectedURLs: []string{"tel"},
		},
		{
			name: "allowed embedded links without double slashes are allowed",
			in: &model.Post{
				Message: "[plugin@example.com](mailto:plugin@example.com)",
			},
			expectedURLs: []string{},
		},
		{
			name: "allowed embedded links with double slashes are allowed",
			in: &model.Post{
				Message: "[plugin@example.com](mailto://plugin@example.com)",
			},
			expectedURLs: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedURLs := p.extractURLs(test.in)
			invalidURLs := p.getInvalidURLs(detectedURLs, test.in)
			assert.ElementsMatch(t, test.expectedURLs, invalidURLs)
		})
	}
}

func TestGetInvalidURLsWithoutPlainLinks(t *testing.T) {
	p := newTestPlugin(false, "http,https,mailto", "http,https,mailto", "")

	var tests = []struct {
		name         string
		in           *model.Post
		expectedURLs []string
	}{
		{
			name: "allows plain link with double slashes",
			in: &model.Post{
				Message: "tel://999999999",
			},
			expectedURLs: []string{},
		},
		{
			name: "allows plain link without double slashes",
			in: &model.Post{
				Message: "tel:999999999",
			},
			expectedURLs: []string{},
		},
		{
			name: "allows embedded link without double slashes",
			in: &model.Post{
				Message: "[+999999999](tel:+999999999)",
			},
			expectedURLs: []string{"tel"},
		},
		{
			name: "allows embedded link without double slashes",
			in: &model.Post{
				Message: "[plugin@example.com](mailto:plugin@example.com)",
			},
			expectedURLs: []string{},
		},
		{
			name: "allows embedded link with double slashes",
			in: &model.Post{
				Message: "[plugin@example.com](mailto://plugin@example.com)",
			},
			expectedURLs: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedURLs := p.extractURLs(test.in)
			invalidURLs := p.getInvalidURLs(detectedURLs, test.in)
			assert.ElementsMatch(t, test.expectedURLs, invalidURLs)
		})
	}
}
