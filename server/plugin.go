package main

import (
	"fmt"
	"regexp"
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
	configuration *configuration
	linkRegex *regexp.Regexp
	allowedProtocolsRegex *regexp.Regexp
}

func (p *Plugin) OnActivate() error {
	regexString :=`\[(?P<text>.*?)\]\((?P<protocol>\w+)://(?P<host>[^\n)]+)?\)`
	regex, err := regexp.Compile(regexString)
	if err != nil {
		return err
	}

	p.linkRegex = regex

	return nil
}

func (p *Plugin) FilterPost(post *model.Post) (*model.Post, string) {
	configuration := p.getConfiguration()

	postText := []byte(post.Message)
	detectedURLProtocols := p.linkRegex.FindAllSubmatchIndex(postText, -1)

	for _, loc := range detectedURLProtocols {
		protocol := string(postText[loc[4]:loc[5]])
		host := string(postText[loc[6]:loc[7]])

		if !p.allowedProtocolsRegex.MatchString(protocol) {
			p.API.SendEphemeralPost(post.UserId, &model.Post{
				ChannelId: post.ChannelId,
				Message:   fmt.Sprintf(configuration.WarningMessage, fmt.Sprintf("%s://%s", protocol, host)),
				RootId:    post.RootId,
			})
			return nil, fmt.Sprintf("Protocol not allowed: %s", protocol)
		}
	}

	return post, ""
}

func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	return p.FilterPost(post)
}

func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost *model.Post, _ *model.Post) (*model.Post, string) {
	return p.FilterPost(newPost)
}
