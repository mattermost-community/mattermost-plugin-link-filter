package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestGetInvalidURLs(t *testing.T) {
	p := Plugin{
		configuration: &configuration{
			RejectPlainLinks:    true,
			AllowedProtocolList: "http,https,mailto",
		},
	}
	p.plainLinkRegex = regexp.MustCompile(PlainLinkRegexString)
	p.embeddedLinkRegex = regexp.MustCompile(EmbeddedLinkRegexString)
	p.allowedProtocolsRegex = regexp.MustCompile(wordListToRegex(p.getConfiguration().AllowedProtocolList))

	t.Run("link protocol matches allowed protocol list", func(t *testing.T) {
		in := &model.Post{
			Message: "[test](https://www.github.com)",
		}
		invalidURLs := p.getInvalidURLs(in)
		assert.ElementsMatch(t, []string{}, invalidURLs)
	})

	t.Run("link protocol invalid", func(t *testing.T) {
		in := &model.Post{
			Message: "[test](s3://www.github.com)",
		}
		invalidURLs := p.getInvalidURLs(in)
		expectedURLs := []string{"s3"}
		assert.ElementsMatch(t, expectedURLs, invalidURLs)
	})

	t.Run("multiple link protocols invalid", func(t *testing.T) {
		in := &model.Post{
			Message: "[test](s3://www.github.com) [test](s4://www.github.com) [test](s3://www.github.com)",
		}
		invalidURLs := p.getInvalidURLs(in)
		expectedURLs := []string{"s3", "s4"}
		assert.ElementsMatch(t, expectedURLs, invalidURLs)
	})

	t.Run("invalid plain text link", func(t *testing.T) {
		in := &model.Post{
			Message: "s3://www.github.com [test](s4://www.github.com)",
		}
		invalidURLs := p.getInvalidURLs(in)
		expectedURLs := []string{"s3", "s4"}
		assert.ElementsMatch(t, expectedURLs, invalidURLs)
	})

	t.Run("message with valid and invalid links", func(t *testing.T) {
		in := &model.Post{
			Message: "https://www.github.com [test](s3://www.github.com)",
		}
		invalidURLs := p.getInvalidURLs(in)
		expectedURLs := []string{"s3"}
		assert.ElementsMatch(t, expectedURLs, invalidURLs)
	})
}
