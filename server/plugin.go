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
	PlainLinkRegexString = `(?P<protocol>\w+):(?://|)(?P<host>[^\n\s),]+)`

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
	message := post.Message

	// Special cases: Different backtick patterns with protocols we care about

	// Case 1: Fully backticked content like "`tel:123456`"
	if strings.HasPrefix(message, "`") && strings.HasSuffix(message, "`") {
		content := message[1 : len(message)-1]
		for _, protocol := range p.rewriteProtocolList {
			if strings.HasPrefix(content, protocol+":") {
				// Already correctly backticked, no need to extract
				return []*detectedURL{}
			}
		}
	}

	// Case 2: Message has backtick at end like "tel:123456`"
	for _, protocol := range p.rewriteProtocolList {
		if strings.HasPrefix(message, protocol+":") && strings.HasSuffix(message, "`") {
			// Has backtick at end, will be handled by rewriteLinks
			return []*detectedURL{}
		}
	}

	// Case 3: Message has backtick at beginning like "`tel:123456"
	for _, protocol := range p.rewriteProtocolList {
		if strings.HasPrefix(message, "`"+protocol+":") {
			// Has backtick at beginning, will be handled by rewriteLinks
			return []*detectedURL{}
		}
	}

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
func (p *Plugin) getInvalidProtocols(detectedURLs []*detectedURL, post *model.Post) []string {
	configuration := p.getConfiguration()

	// Special handling for backticked messages containing protocols in the rewrite list
	msg := post.Message

	// Case 1: Completely backticked content like "`tel:123456`"
	if strings.HasPrefix(msg, "`") && strings.HasSuffix(msg, "`") {
		// Extract content between backticks
		content := msg[1 : len(msg)-1]
		for _, protocol := range p.rewriteProtocolList {
			if strings.HasPrefix(content, protocol+":") {
				// This is a valid and already properly backticked protocol
				return []string{}
			}
		}
	}

	// Case 2: Single backtick at the end like "tel:123456`"
	for _, protocol := range p.rewriteProtocolList {
		if strings.HasPrefix(msg, protocol+":") && strings.HasSuffix(msg, "`") {
			// This is "tel:123456`" style
			return []string{}
		}
	}

	// Case 3: Single backtick at the beginning like "`tel:123456"
	for _, protocol := range p.rewriteProtocolList {
		if strings.HasPrefix(msg, "`"+protocol+":") {
			// This is "`tel:123456" style
			return []string{}
		}
	}

	var invalidURLProtocols []string
	set := make(map[string]struct{})

	for _, u := range detectedURLs {
		// Skip if the URL has already been rewritten
		if u.rewritten {
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
// and rewrites them to backticks to prevent autolinking. Special care is taken for messages that already have backticks.
func (p *Plugin) rewriteLinks(detectedURLs []*detectedURL, post *model.Post) string {
	msg := post.Message

	// If we have no URLs to process, return the original message
	if len(detectedURLs) == 0 {
		return msg
	}

	// Messages that are already potentially backticked
	if strings.HasPrefix(msg, "`") && strings.HasSuffix(msg, "`") {
		// Message is already completely backticked
		return msg
	}

	// Special case for URL with backtick only at the end or the beginning
	for _, protocol := range p.rewriteProtocolList {
		// Check for backtick at the end - "tel:123456`"
		if strings.HasPrefix(msg, protocol+":") && strings.HasSuffix(msg, "`") {
			// This is "tel:123456`" style - need to properly backtick it
			content := msg[:len(msg)-1] // Remove trailing backtick
			return "`" + content + "`"
		}

		// Check for backtick at the beginning - "`tel:123456"
		if strings.HasPrefix(msg, "`"+protocol+":") {
			// This is "`tel:123456" style - need to properly backtick it
			content := msg[1:] // Remove leading backtick
			return "`" + content + "`"
		}
	}

	// Normal processing for everything else
	var builder strings.Builder
	lastIndex := 0

	for i, u := range detectedURLs {
		if u.isPlainText && slices.Contains(p.rewriteProtocolList, u.protocol) {
			detectedURLs[i].rewritten = true
			backticked := "`" + u.originalText + "`"

			// Append the text before the detected URL
			builder.WriteString(msg[lastIndex:u.positions[0]])
			// Append the backticked URL
			builder.WriteString(backticked)
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
