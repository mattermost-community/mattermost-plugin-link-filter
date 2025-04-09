package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

type detectedURL struct {
	protocol     string
	host         string
	originalText string
	isPlainText  bool
	positions    []int
	rewritten    bool
}

type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration                  *configuration
	embeddedLinkRegex              *regexp.Regexp
	plainLinkRegex                 *regexp.Regexp
	allowedProtocolsRegexLink      *regexp.Regexp
	allowedProtocolsRegexPlainText *regexp.Regexp
	rewriteProtocolList            []string
}

const (
	// Following regex would match links embedded with texts in markdown
	// e.g. [test](https://www.github.com)
	EmbeddedLinkRegexString = `\[(?P<text>.*?)\]\((?P<protocol>\w+):[//]?(?P<host>[^\n)]+)?\)`

	// Following regex would match links
	// e.g. https://github.com
	PlainLinkRegexString = `(?P<protocol>\w+):[//]?(?P<host>[^\n\s)]+)?`

	// Message to be displayed when a post is rejected
	InvalidURLSchemeMessage = "\nFollowing URL Scheme is not allowed: `%s`"
)

func (p *Plugin) OnActivate() error {
	p.embeddedLinkRegex = regexp.MustCompile(EmbeddedLinkRegexString)
	p.plainLinkRegex = regexp.MustCompile(PlainLinkRegexString)

	return nil
}

func (p *Plugin) extractURLs(post *model.Post) []*detectedURL {
	postText := []byte(post.Message)
	detectedURLs := []*detectedURL{}
	embeddedLinks := p.embeddedLinkRegex.FindAllSubmatchIndex(postText, -1)

	// loc contains the index of relevant groups
	// [0-1] start and end position of entire match
	// [2-3] start and end position of "scheme" or "text"
	// [4-5] start and end position of "scheme" or "host"
	// [6-7] start and end position of "host"

	for _, loc := range embeddedLinks {
		p.API.LogError("loc1", "loc", loc)
		detectedURLs = append(detectedURLs, &detectedURL{
			protocol:     string(postText[loc[4]:loc[5]]),
			host:         string(postText[loc[6]:loc[7]]),
			originalText: string(postText[loc[0]:loc[1]]),
			positions:    loc,
		})
	}

	plainLinks := p.plainLinkRegex.FindAllSubmatchIndex(postText, -1)
	for _, loc := range plainLinks {
		p.API.LogError("loc2", "loc", loc)
		detectedURLs = append(detectedURLs, &detectedURL{
			protocol:     string(postText[loc[2]:loc[3]]),
			host:         string(postText[loc[4]:loc[5]]),
			originalText: string(postText[loc[0]:loc[1]]),
			positions:    loc,
		})
	}

	return detectedURLs
}

func (p *Plugin) getInvalidURLs(detectedURLs []*detectedURL, post *model.Post) []string {
	configuration := p.getConfiguration()

	var invalidURLProtocols []string
	set := make(map[string]struct{})

	p.API.LogError("detectedURLs", "detectedURLs", detectedURLs)
	for _, u := range detectedURLs {

		// Skip if the URL has already been rewritten
		if u.rewritten {
			continue
		}

		// If protocol is banned
		_, ok := set[u.protocol]
		if !ok && (len(configuration.AllowedProtocolListLink) == 0 || !p.allowedProtocolsRegexLink.MatchString(u.protocol)) {
			invalidURLProtocols = append(invalidURLProtocols, u.protocol)
			set[u.protocol] = struct{}{}
		}
		if !ok && configuration.RejectPlainLinks && u.isPlainText && (len(configuration.AllowedProtocolListPlainText) == 0 || !p.allowedProtocolsRegexPlainText.MatchString(u.protocol)) {
			invalidURLProtocols = append(invalidURLProtocols, u.protocol)
			set[u.protocol] = struct{}{}
		}
	}

	return invalidURLProtocols
}

func (p *Plugin) FilterPost(detectedURLs []*detectedURL, post *model.Post, isEdit bool) (*model.Post, string) {
	configuration := p.getConfiguration()

	invalidURLProtocols := p.getInvalidURLs(detectedURLs, post)
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

func (p *Plugin) rewriteLinks(detectedURLs []*detectedURL, post *model.Post) string {
	postText := []byte(post.Message)
	delta := 0
	for _, u := range detectedURLs {
		if u.isPlainText && slices.Contains(p.rewriteProtocolList, u.protocol) {
			u.rewritten = true
			p.API.LogError("rewrite", "protocol", u.protocol, "u", u)
			p.API.LogError("postText", "originalText", string(postText))

			backticked := "`" + u.originalText + "`"

			// Why not just use `bytes.Replace`?
			// Replacing the text would not work in this case because the URL could be in the same message several times. This way
			// we ensure that we maintain the original message format by cutting the captured link and replacing it with the backticked
			// version.
			postText = append(postText[0:u.positions[0]+delta], append([]byte(backticked), postText[u.positions[1]+delta:]...)...)
			delta += 2 // The two backticks we add to the original text
		}
	}

	p.API.LogError("postText", "postText", string(postText))

	return string(postText)
}

func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	detectedURLs := p.extractURLs(post)
	post.Message = p.rewriteLinks(detectedURLs, post)
	return p.FilterPost(detectedURLs, post, false)
}

func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost *model.Post, oldPost *model.Post) (*model.Post, string) {
	detectedURLs := p.extractURLs(newPost)
	newPost.Message = p.rewriteLinks(detectedURLs, newPost)
	return p.FilterPost(detectedURLs, newPost, true)
}
