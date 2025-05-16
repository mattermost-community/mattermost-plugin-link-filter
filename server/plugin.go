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
	EmbeddedLinkRegexString = `\[(?P<text>.*?)\]\((?P<protocol>\w+):(?://|)(?P<host>[^\n\s)]+)\)`

	// Following regex would match links
	// e.g. https://github.com
	// Note: Ensures we don't match trailing characters like commas in URLs
	// But preserves special characters like + in the URL
	PlainLinkRegexString = `(?P<protocol>\w+):(?://|)(?P<host>[^\n\s),` + "`" + `]+)`

	// Message to be displayed when a post is rejected
	InvalidURLSchemeMessage = "\nFollowing URL Scheme is not allowed: `%s`"
)

func (p *Plugin) OnActivate() error {
	p.initRegexes()

	return nil
}

func (p *Plugin) initRegexes() {
	p.embeddedLinkRegex = regexp.MustCompile(EmbeddedLinkRegexString)
	p.plainLinkRegex = regexp.MustCompile(PlainLinkRegexString)
}

// extractURLs extracts the URLs from the post using regular expressions.
func (p *Plugin) extractURLs(post *model.Post) []*detectedURL {
	postText := []byte(post.Message)
	detectedURLs := []*detectedURL{}
	embeddedLinks := p.embeddedLinkRegex.FindAllSubmatchIndex(postText, -1)

	// loc contains the index of relevant groups
	// [0-1] start and end position of entire match
	// [2-3] start and end position of "scheme" (plain) or "text" (markdown)
	// [4-5] start and end position of "scheme" (markdown) or "host" (plain)
	// [6-7] start and end position of "host" (markdown)

	for _, loc := range embeddedLinks {
		detectedURLs = append(detectedURLs, &detectedURL{
			protocol:     string(postText[loc[4]:loc[5]]),
			host:         string(postText[loc[6]:loc[7]]),
			originalText: string(postText[loc[0]:loc[1]]),
			positions:    loc,
		})
	}

	plainLinks := p.plainLinkRegex.FindAllSubmatchIndex(postText, -1)
	for _, loc := range plainLinks {
		// Skip if the URL starts with a parenthesis, which should be captured by the embedded link regex
		if loc[0] > 0 && string(postText[loc[0]-1]) == "(" {
			continue
		}

		detectedURLs = append(detectedURLs, &detectedURL{
			protocol:     string(postText[loc[2]:loc[3]]),
			host:         string(postText[loc[4]:loc[5]]),
			originalText: string(postText[loc[0]:loc[1]]),
			positions:    loc,
			isPlainText:  true,
		})
	}

	return detectedURLs
}

// getInvalidProtocols returns the protocols that are not allowed in the post from the extracted URLs and the
// plugin configuration.
func (p *Plugin) getInvalidProtocols(detectedURLs []*detectedURL, _ *model.Post) []string {
	configuration := p.getConfiguration()

	var invalidURLProtocols []string
	set := make(map[string]struct{})

	for _, u := range detectedURLs {
		// Skip if the URL has already been rewritten
		if u.rewritten {
			continue
		}

		// If it's a rewritable protocol in plain text format, mark it valid
		if u.isPlainText && slices.Contains(p.rewriteProtocolList, u.protocol) {
			u.rewritten = true
			continue
		}

		// If protocol is banned
		_, alreadyPassed := set[u.protocol]
		if !alreadyPassed && !u.isPlainText && (len(configuration.AllowedProtocolListLink) == 0 || !p.allowedProtocolsRegexLink.MatchString(u.protocol)) {
			invalidURLProtocols = append(invalidURLProtocols, u.protocol)
			set[u.protocol] = struct{}{}
		} else if !alreadyPassed && configuration.RejectPlainLinks && u.isPlainText && (len(configuration.AllowedProtocolListPlainText) == 0 || !p.allowedProtocolsRegexPlainText.MatchString(u.protocol)) {
			invalidURLProtocols = append(invalidURLProtocols, u.protocol)
			set[u.protocol] = struct{}{}
		}
	}

	return invalidURLProtocols
}

// FilterPost filters the post based on the plugin configuration.
// If the post is rejected, it sends an ephemeral post to the user and returns the error message with a nil post.
func (p *Plugin) FilterPost(detectedURLs []*detectedURL, post *model.Post, isEdit bool) string {
	configuration := p.getConfiguration()

	invalidURLProtocols := p.getInvalidProtocols(detectedURLs, post)
	if len(invalidURLProtocols) == 0 {
		return ""
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

	return fmt.Sprintf("Schemes not allowed: %s", strings.Join(invalidURLProtocols, ", "))
}

// rewriteLinks rewrites the links in the post based on the plugin configuration. Finds which plain links are allowed to be rewritten
// and rewrites them to prevent autolinking. Special care is taken for messages that already have backticks.
func (p *Plugin) rewriteLinks(detectedURLs []*detectedURL, post *model.Post) string {
	msg := post.Message

	// If we have no URLs to process, return the original message
	if len(detectedURLs) == 0 {
		return msg
	}

	// Normal processing for everything else
	var builder strings.Builder
	lastIndex := 0

	for i, u := range detectedURLs {
		if u.isPlainText && slices.Contains(p.rewriteProtocolList, u.protocol) {
			detectedURLs[i].rewritten = true
			// Trim any leading "//" from the host part
			host := strings.TrimPrefix(u.host, "//")

			rewritten := fmt.Sprintf("%s(%s)", u.protocol, host)

			// Append the text before the detected URL
			builder.WriteString(msg[lastIndex:u.positions[0]])
			// Append the rewritten URL
			builder.WriteString(rewritten)
			// Update the last index to the end of the current URL
			lastIndex = u.positions[1]
		}
	}

	// Append the remaining text after the last URL
	builder.WriteString(msg[lastIndex:])
	return builder.String()
}

func (p *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	detectedURLs := p.extractURLs(post)
	post.Message = p.rewriteLinks(detectedURLs, post)

	if errMessage := p.FilterPost(detectedURLs, post, false); errMessage != "" {
		return nil, errMessage
	}

	return post, ""
}

func (p *Plugin) MessageWillBeUpdated(_ *plugin.Context, newPost *model.Post, _ *model.Post) (*model.Post, string) {
	detectedURLs := p.extractURLs(newPost)
	newPost.Message = p.rewriteLinks(detectedURLs, newPost)

	if errMessage := p.FilterPost(detectedURLs, newPost, true); errMessage != "" {
		return nil, errMessage
	}

	return newPost, ""
}
