package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration         *configuration
	embeddedLinkRegex     *regexp.Regexp
	plainLinkRegex        *regexp.Regexp
	allowedProtocolsRegex *regexp.Regexp
}

const (
	// Following regex would match links embedded with texts in markdown
	// e.g. [test](https://www.github.com)
	EmbeddedLinkRegexString = `\[(?P<text>.*?)\]\((?P<protocol>\w+)://(?P<host>[^\n)]+)?\)`

	// Following regex would match links
	// e.g. https://github.com
	PlainLinkRegexString = `(?P<protocol>\w+)://(?P<host>[^\n)]+)?`

	InvalidURLSchemeMessage = "\nFollowing URL Scheme is not allowed: `%s`"
)

func (p *Plugin) OnActivate() error {
	embeddedLinkRegex, err := regexp.Compile(EmbeddedLinkRegexString)
	if err != nil {
		return err
	}

	plainLinkRegex, err := regexp.Compile(PlainLinkRegexString)
	if err != nil {
		return err
	}

	p.embeddedLinkRegex = embeddedLinkRegex
	p.plainLinkRegex = plainLinkRegex
	return nil
}

func (p *Plugin) FilterPost(post *model.Post, isEdit bool) (*model.Post, string) {
	configuration := p.getConfiguration()

	postText := []byte(post.Message)
	detectedURLs := p.embeddedLinkRegex.FindAllSubmatchIndex(postText, -1)
	if configuration.RejectPlainLinks {
		plainLinks := p.plainLinkRegex.FindAllSubmatchIndex(postText, -1)
		detectedURLs = append(detectedURLs, plainLinks...)
	}
	var invalidURLProtocols []string
	set := make(map[string]bool)

	for _, loc := range detectedURLs {
		// loc contains the index of relevant groups
		// [0-1] start and end position of regex
		// [2-3] start and end position of "text"
		// [4-5] start and end position of "protocol"
		// [6-7] start and end position of "host"
		protocolStartIndex := loc[4]
		procolEndIndex := loc[5]

		// The case when detected url has length 6 i.e., the url is plain link
		// then the detected url will have no "text" and
		// start and end position of "protocol" will be 2-3 and not 4-5
		if len(loc) == 6 {
			protocolStartIndex = loc[2]
			procolEndIndex = loc[3]
		}

		protocol := string(postText[protocolStartIndex:procolEndIndex])
		_, ok := set[protocol]
		if !ok && (len(configuration.AllowedProtocolList) == 0 || !p.allowedProtocolsRegex.MatchString(protocol)) {
			invalidURLProtocols = append(invalidURLProtocols, protocol)
			set[protocol] = true
		}
	}

	if len(invalidURLProtocols) == 0 {
		return post, ""
	}

	WarningMessage := configuration.CreatePostWarningMessage
	if isEdit {
		WarningMessage = configuration.EditPostWarningMessage
	}
	WarningMessage += fmt.Sprintf(InvalidURLSchemeMessage, strings.Join(invalidURLProtocols, ", "))
	p.API.SendEphemeralPost(post.UserId, &model.Post{
		ChannelId: post.ChannelId,
		Message:   WarningMessage,
		RootId:    post.RootId,
	})
	return nil, fmt.Sprintf("Schemes not allowed: %s", strings.Join(invalidURLProtocols, ", "))
}

func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	return p.FilterPost(post, false)
}

func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost *model.Post, oldPost *model.Post) (*model.Post, string) {
	post, err := p.FilterPost(newPost, true)
	if err != "" {
		return oldPost, err
	}
	return post, err
}
