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

func (p *Plugin) OnActivate() error {
	regexString := `\[(?P<text>.*?)\]\((?P<protocol>\w+)://(?P<host>[^\n)]+)?\)`
	embeddedLinkRegex, err := regexp.Compile(regexString)
	if err != nil {
		return err
	}

	regexString = `(?P<protocol>\w+)://(?P<host>[^\n)]+)?`
	plainLinkRegex, err := regexp.Compile(regexString)
	if err != nil {
		return err
	}

	p.embeddedLinkRegex = embeddedLinkRegex
	p.plainLinkRegex = plainLinkRegex
	return nil
}

func (p *Plugin) FilterPost(post *model.Post) (*model.Post, string) {
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
		protocol := string(postText[loc[4]:loc[5]])

		// The case when detected url has length 6 i.e., the url is plain link
		// then the detected url will have no "text" and
		// start and end position of "protocol" will be 2-3 and not 4-5
		if len(loc) == 6 {
			protocol = string(postText[loc[2]:loc[3]])
		}
		_, ok := set[protocol]
		if !ok && (len(configuration.AllowedProtocolList) == 0 || !p.allowedProtocolsRegex.MatchString(protocol)) {
			invalidURLProtocols = append(invalidURLProtocols, protocol)
			set[protocol] = true
		}
	}

	if len(invalidURLProtocols) == 0 {
		return post, ""
	}

	p.API.SendEphemeralPost(post.UserId, &model.Post{
		ChannelId: post.ChannelId,
		Message:   fmt.Sprintf(configuration.WarningMessage, strings.Join(invalidURLProtocols, ", ")),
		RootId:    post.RootId,
	})
	return nil, fmt.Sprintf("Schemes not allowed: %s", strings.Join(invalidURLProtocols, ", "))
}

func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	return p.FilterPost(post)
}

func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost *model.Post, _ *model.Post) (*model.Post, string) {
	return p.FilterPost(newPost)
}
