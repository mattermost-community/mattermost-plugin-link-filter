package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// Helper function to create a test plugin with the given configuration
func newTestPlugin(t *testing.T, rejectPlainLinks bool, allowedProtocolListLink, allowedProtocolListPlainText, rewriteProtocolList string) *Plugin {
	p := &Plugin{}
	p.configuration = &configuration{
		RejectPlainLinks:             rejectPlainLinks,
		AllowedProtocolListLink:      allowedProtocolListLink,
		AllowedProtocolListPlainText: allowedProtocolListPlainText,
		RewriteProtocolList:          rewriteProtocolList,
		CreatePostWarningMessage:     "Your post has been rejected by the Link Filter.",
		EditPostWarningMessage:       "Your edit has been rejected by the Link Filter.",
	}

	// Initialize regex patterns
	p.initRegexes()
	require.NoError(t, p.initConfiguration(p.configuration))

	return p
}

// TestExtractURLs tests the extractURLs method
func TestExtractURLs(t *testing.T) {
	// Test with empty allowed protocols since that shouldn't matter for the URL extraction
	p := newTestPlugin(t, true, "", "", "")

	var tests = []struct {
		name          string
		in            *model.Post
		expectedCount int
		expectedURLs  []*detectedURL
	}{
		{
			name: "extracts embedded link",
			in: &model.Post{
				Message: "[test](https://www.github.com)",
			},
			expectedCount: 1,
			expectedURLs: []*detectedURL{
				{
					protocol:     "https",
					host:         "www.github.com",
					originalText: "[test](https://www.github.com)",
					isPlainText:  false,
				},
			},
		},
		{
			name: "extracts plain link",
			in: &model.Post{
				Message: "https://www.github.com",
			},
			expectedCount: 1,
			expectedURLs: []*detectedURL{
				{
					protocol:     "https",
					host:         "www.github.com",
					originalText: "https://www.github.com",
					isPlainText:  true,
				},
			},
		},
		{
			name: "extracts multiple links",
			in: &model.Post{
				Message: "[test](s3://www.github.com) https://www.github.com",
			},
			expectedCount: 2,
			expectedURLs: []*detectedURL{
				{
					protocol:     "s3",
					host:         "www.github.com",
					originalText: "[test](s3://www.github.com)",
					isPlainText:  false,
				},
				{
					protocol:     "https",
					host:         "www.github.com",
					originalText: "https://www.github.com",
					isPlainText:  true,
				},
			},
		},
		{
			name: "extracts mailto link",
			in: &model.Post{
				Message: "[plugin@example.com](mailto:plugin@example.com)",
			},
			expectedCount: 1,
			expectedURLs: []*detectedURL{
				{
					protocol:     "mailto",
					host:         "plugin@example.com",
					originalText: "[plugin@example.com](mailto:plugin@example.com)",
					isPlainText:  false,
				},
			},
		},
		{
			name: "extracts tel link",
			in: &model.Post{
				Message: "tel://999999999",
			},
			expectedCount: 1,
			expectedURLs: []*detectedURL{
				{
					protocol:     "tel",
					host:         "999999999",
					originalText: "tel://999999999",
					isPlainText:  true,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedURLs := p.extractURLs(test.in)
			assert.Equal(t, test.expectedCount, len(detectedURLs))
			// Compare the extracted URLs with the expected URLs. Mind that the order of the URLs is not guaranteed.
			for _, expectedURL := range test.expectedURLs {
				found := false
				for _, detectedURL := range detectedURLs {
					if expectedURL.originalText == detectedURL.originalText {
						// Ensure that we extracted the URL correctly
						require.Equal(t, expectedURL.protocol, detectedURL.protocol, "Protocol mismatch for %s", expectedURL.originalText)
						require.Equal(t, expectedURL.host, detectedURL.host, "Host mismatch for %s", expectedURL.originalText)
						require.Equal(t, expectedURL.isPlainText, detectedURL.isPlainText, "isPlainText mismatch for %s", expectedURL.originalText)
						found = true
						break
					}
				}
				assert.True(t, found, "%s not found in detected URLs", expectedURL.originalText)
			}
		})
	}
}

func TestGetInvalidURLsWithPlainLinks(t *testing.T) {
	p := newTestPlugin(t, true, "http,https,mailto", "http,https,mailto", "")

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
			invalidURLs := p.getInvalidProtocols(detectedURLs, test.in)
			assert.ElementsMatch(t, test.expectedURLs, invalidURLs)
		})
	}
}

func TestGetInvalidURLsWithRewriteProtocols(t *testing.T) {
	p := newTestPlugin(t, true, "http,https,mailto", "http,https,mailto", "tel,ftp")

	var tests = []struct {
		name             string
		in               *model.Post
		invalidProtocols []string
	}{
		{
			name: "allows tel protocol in plain link due to rewrite list",
			in: &model.Post{
				Message: "tel://999999999",
			},
			invalidProtocols: []string{},
		},
		{
			name: "rejects tel protocol in embedded link",
			in: &model.Post{
				Message: "[test](tel://999999999)",
			},
			invalidProtocols: []string{"tel"},
		},
		{
			name: "allows ftp protocol in plain link due to rewrite list",
			in: &model.Post{
				Message: "ftp://example.com",
			},
			invalidProtocols: []string{},
		},
		{
			name: "rejects tel in embedded link but allows tel in plain text",
			in: &model.Post{
				Message: "tel:123456 [test](tel://999)",
			},
			invalidProtocols: []string{"tel"},
		},
		{
			name: "reject rewriten protocol in markdown link",
			in: &model.Post{
				Message: "ftp://example.com [test](tel://999999999)",
			},
			invalidProtocols: []string{"tel"},
		},
		{
			name: "rejects unlisted protocol in markdown link",
			in: &model.Post{
				Message: "sftp://example.com",
			},
			invalidProtocols: []string{"sftp"},
		},
		{
			name: "reject unlisted protocol in plain link",
			in: &model.Post{
				Message: "aria2://999999999",
			},
			invalidProtocols: []string{"aria2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedURLs := p.extractURLs(test.in)
			_ = p.rewriteLinks(detectedURLs, test.in)
			invalidProtocols := p.getInvalidProtocols(detectedURLs, test.in)
			assert.ElementsMatch(t, test.invalidProtocols, invalidProtocols)
		})
	}
}

// TestRewriteLinks tests the rewriteLinks method
func TestRewriteLinks(t *testing.T) {
	p := newTestPlugin(t, true, "http,https,mailto", "http,https,mailto", "tel,ftp")

	t.Run("rewritting a link marks it as rewritten", func(t *testing.T) {
		detectedURLs := p.extractURLs(&model.Post{
			Message: "tel://999999999",
		})
		rewrittenMessage := p.rewriteLinks(detectedURLs, &model.Post{
			Message: "tel://999999999",
		})
		assert.Equal(t, "`tel://999999999`", rewrittenMessage)
		assert.True(t, detectedURLs[0].rewritten)
	})

	t.Run("test rewriting", func(t *testing.T) {
		var tests = []struct {
			name           string
			in             *model.Post
			expectedOutput string
		}{
			{
				name: "rewrites tel protocol in plain link",
				in: &model.Post{
					Message: "tel://999999999",
				},
				expectedOutput: "`tel://999999999`",
			},
			{
				name: "rewrites ftp protocol in plain link",
				in: &model.Post{
					Message: "ftp://example.com",
				},
				expectedOutput: "`ftp://example.com`",
			},
			{
				name: "rewrites multiple protocols in mixed content",
				in: &model.Post{
					Message: "tel:123456 [test](ftp://example.com) [test2](tel://999)",
				},
				expectedOutput: "`tel:123456` [test](ftp://example.com) [test2](tel://999)",
			},
			{
				name: "doesn't rewrite non-listed protocols",
				in: &model.Post{
					Message: "sftp://example.com",
				},
				expectedOutput: "sftp://example.com",
			},
			{
				name: "rewrites multiple occurrences of the same protocol",
				in: &model.Post{
					Message: "tel:123456 tel:789012",
				},
				expectedOutput: "`tel:123456` `tel:789012`",
			},
			{
				name: "multiple occurrences of the same protocol and value",
				in: &model.Post{
					Message: "tel:123456 tel:789012 tel:123456",
				},
				expectedOutput: "`tel:123456` `tel:789012` `tel:123456`",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				detectedURLs := p.extractURLs(test.in)
				rewrittenMessage := p.rewriteLinks(detectedURLs, test.in)
				assert.Equal(t, test.expectedOutput, rewrittenMessage)
			})
		}
	})
}

// Mock API implementation
type mockAPI struct {
	plugin.API
	sentEphemeralPost *model.Post
}

func (m *mockAPI) SendEphemeralPost(_ string, post *model.Post) *model.Post {
	m.sentEphemeralPost = post
	return post
}

// TestFilterPost tests the FilterPost method
func TestFilterPost(t *testing.T) {
	p := newTestPlugin(t, true, "http,https,mailto", "http,https,mailto", "tel")
	mockAPI := &mockAPI{}
	p.API = mockAPI

	var tests = []struct {
		name           string
		in             *model.Post
		isEdit         bool
		expectedPost   *model.Post
		expectedError  string
		checkEphemeral func(*testing.T, *model.Post)
	}{
		{
			name: "allows valid post",
			in: &model.Post{
				Message:   "[test](https://www.github.com)",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			isEdit: false,
			expectedPost: &model.Post{
				Message:   "[test](https://www.github.com)",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedError: "",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.Nil(t, p, "No ephemeral post should be sent for valid posts")
			},
		},
		{
			name: "rejects post with invalid protocol",
			in: &model.Post{
				Message:   "[test](s3://www.github.com)",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			isEdit:        false,
			expectedPost:  nil,
			expectedError: "Schemes not allowed: s3",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.NotNil(t, p, "Ephemeral post should be sent for invalid posts")
				assert.Equal(t, "channel1", p.ChannelId)
				assert.Contains(t, p.Message, "s3")
			},
		},
		{
			name: "rejects edit with invalid protocol",
			in: &model.Post{
				Message:   "[test](s3://www.github.com)",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			isEdit:        true,
			expectedPost:  nil,
			expectedError: "Schemes not allowed: s3",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.NotNil(t, p, "Ephemeral post should be sent for invalid edits")
				assert.Equal(t, "channel1", p.ChannelId)
				assert.Contains(t, p.Message, "s3")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAPI.sentEphemeralPost = nil // Reset for each test
			detectedURLs := p.extractURLs(test.in)
			post, errString := p.FilterPost(detectedURLs, test.in, test.isEdit)

			if test.expectedPost == nil {
				require.Nil(t, post)
				assert.Equal(t, test.expectedError, errString)
			} else {
				require.Empty(t, errString)
				assert.Equal(
					t,
					test.expectedPost.Message,
					post.Message)
				assert.Equal(t, test.expectedPost.UserId, post.UserId)
				assert.Equal(t, test.expectedPost.ChannelId, post.ChannelId)
			}

			if test.checkEphemeral != nil {
				test.checkEphemeral(t, mockAPI.sentEphemeralPost)
			}
		})
	}
}

// TestRegexPatterns tests the regex patterns directly
func TestRegexPatterns(t *testing.T) {
	embeddedRegex := regexp.MustCompile(EmbeddedLinkRegexString)
	plainRegex := regexp.MustCompile(PlainLinkRegexString)

	tests := []struct {
		name         string
		input        string
		regex        *regexp.Regexp
		expectMatch  bool
		expectGroups map[string]string
	}{
		{
			name:        "embedded link with https",
			input:       "[test](https://www.github.com)",
			regex:       embeddedRegex,
			expectMatch: true,
			expectGroups: map[string]string{
				"text":     "test",
				"protocol": "https",
				"host":     "www.github.com",
			},
		},
		{
			name:        "embedded link with s3",
			input:       "[test](s3://bucket.name)",
			regex:       embeddedRegex,
			expectMatch: true,
			expectGroups: map[string]string{
				"text":     "test",
				"protocol": "s3",
				"host":     "bucket.name",
			},
		},
		{
			name:        "plain link with https",
			input:       "https://www.github.com",
			regex:       plainRegex,
			expectMatch: true,
			expectGroups: map[string]string{
				"protocol": "https",
				"host":     "www.github.com",
			},
		},
		{
			name:        "plain link with tel",
			input:       "tel:999999999",
			regex:       plainRegex,
			expectMatch: true,
			expectGroups: map[string]string{
				"protocol": "tel",
				"host":     "999999999",
			},
		},
		{
			name:        "plain link with mailto",
			input:       "mailto:user@example.com",
			regex:       plainRegex,
			expectMatch: true,
			expectGroups: map[string]string{
				"protocol": "mailto",
				"host":     "user@example.com",
			},
		},
		{
			name:         "Message with semicolons",
			input:        "this is a link: or not",
			regex:        plainRegex,
			expectMatch:  false,
			expectGroups: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := tt.regex.FindStringSubmatch(tt.input)
			if tt.expectMatch {
				require.NotNil(t, matches, "Expected input to match regex but it didn't")

				// Get the named capture group indices
				namedGroups := tt.regex.SubexpNames()

				// Check each expected group
				for groupName, expectedValue := range tt.expectGroups {
					// Find the index of the named group
					groupIdx := -1
					for i, name := range namedGroups {
						if name == groupName {
							groupIdx = i
							break
						}
					}
					require.NotEqual(t, -1, groupIdx, "Named group %s not found in regex", groupName)
					require.Equal(t, expectedValue, matches[groupIdx],
						"Mismatch in group %s. Expected: %s, Got: %s",
						groupName, expectedValue, matches[groupIdx])
				}
			} else {
				require.Nil(t, matches, "Expected input not to match regex but it did")
			}
		})
	}
}

// TestMessageHooks tests the MessageWillBePosted and MessageWillBeUpdated methods
func TestMessageHooks(t *testing.T) {
	// Create a plugin with configuration that allows http, https, mailto protocols
	// and rewrites tel and ftp protocols
	p := newTestPlugin(t, true, "http,https,mailto,aria2", "http,https,mailto,aria2", "tel,ftp")
	mockAPI := &mockAPI{}
	p.API = mockAPI

	// Test cases for both MessageWillBePosted and MessageWillBeUpdated
	testCases := []struct {
		name           string
		post           *model.Post
		oldPost        *model.Post // Only used for MessageWillBeUpdated
		expectedPost   *model.Post
		expectedError  string
		checkEphemeral func(*testing.T, *model.Post)
	}{
		{
			name: "allows valid post with http link",
			post: &model.Post{
				Message:   "Check this link: https://www.github.com",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedPost: &model.Post{
				Message:   "Check this link: https://www.github.com",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedError: "",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.Nil(t, p, "No ephemeral post should be sent for valid posts")
			},
		},
		{
			name: "allows valid post with embedded link",
			post: &model.Post{
				Message:   "Check this [link](https://www.github.com)",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedPost: &model.Post{
				Message:   "Check this [link](https://www.github.com)",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedError: "",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.Nil(t, p, "No ephemeral post should be sent for valid posts")
			},
		},
		{
			name: "rejects post with invalid protocol",
			post: &model.Post{
				Message:   "Check this [link](s3://bucket.name)",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedPost:  nil,
			expectedError: "Schemes not allowed: s3",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.NotNil(t, p, "Ephemeral post should be sent for invalid posts")
				assert.Equal(t, "channel1", p.ChannelId)
				assert.Contains(t, p.Message, "s3")
			},
		},
		{
			name: "rewrites tel protocol in plain link",
			post: &model.Post{
				Message:   "Call me at tel://1234567890",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedPost: &model.Post{
				Message:   "Call me at `tel://1234567890`",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedError: "",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.Nil(t, p, "No ephemeral post should be sent for rewritten posts")
			},
		},
		{
			name: "rewrites ftp protocol in plain link",
			post: &model.Post{
				Message:   "Download from ftp://example.com/file.txt",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedPost: &model.Post{
				Message:   "Download from `ftp://example.com/file.txt`",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedError: "",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.Nil(t, p, "No ephemeral post should be sent for rewritten posts")
			},
		},
		{
			name: "rewrites multiple protocols in mixed content",
			post: &model.Post{
				Message:   "tel:123456 [test](https://example.com) [test2](aria2://999) ftp://example.com",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedPost: &model.Post{
				Message:   "`tel:123456` [test](https://example.com) [test2](aria2://999) `ftp://example.com`",
				UserId:    "user1",
				ChannelId: "channel1",
			},
			expectedError: "",
			checkEphemeral: func(t *testing.T, p *model.Post) {
				assert.Nil(t, p, "No ephemeral post should be sent for rewritten posts")
			},
		},
	}

	// Test MessageWillBePosted
	t.Run("MessageWillBePosted", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mockAPI.sentEphemeralPost = nil // Reset for each test

				// Create a copy of the post to avoid modifying the original
				postCopy := &model.Post{
					Message:   tc.post.Message,
					UserId:    tc.post.UserId,
					ChannelId: tc.post.ChannelId,
				}

				// Call MessageWillBePosted
				resultPost, errString := p.MessageWillBePosted(nil, postCopy)

				// Check results
				if tc.expectedPost == nil {
					require.Nil(t, resultPost)
					assert.Equal(t, tc.expectedError, errString)
				} else {
					require.Empty(t, errString)
					assert.Equal(t, tc.expectedPost.Message, resultPost.Message)
					assert.Equal(t, tc.expectedPost.UserId, resultPost.UserId)
					assert.Equal(t, tc.expectedPost.ChannelId, resultPost.ChannelId)
				}

				if tc.checkEphemeral != nil {
					tc.checkEphemeral(t, mockAPI.sentEphemeralPost)
				}
			})
		}
	})

	// Test MessageWillBeUpdated
	t.Run("MessageWillBeUpdated", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mockAPI.sentEphemeralPost = nil // Reset for each test

				// Create a copy of the post to avoid modifying the original
				postCopy := &model.Post{
					Message:   tc.post.Message,
					UserId:    tc.post.UserId,
					ChannelId: tc.post.ChannelId,
				}

				// Create an old post (can be empty for these tests)
				oldPost := &model.Post{
					Message:   "Old message",
					UserId:    tc.post.UserId,
					ChannelId: tc.post.ChannelId,
				}

				// Call MessageWillBeUpdated
				resultPost, errString := p.MessageWillBeUpdated(nil, postCopy, oldPost)

				// Check results
				if tc.expectedPost == nil {
					require.Nil(t, resultPost)
					assert.Equal(t, tc.expectedError, errString)
				} else {
					require.Empty(t, errString)
					assert.Equal(t, tc.expectedPost.Message, resultPost.Message)
					assert.Equal(t, tc.expectedPost.UserId, resultPost.UserId)
					assert.Equal(t, tc.expectedPost.ChannelId, resultPost.ChannelId)
				}

				if tc.checkEphemeral != nil {
					tc.checkEphemeral(t, mockAPI.sentEphemeralPost)
				}
			})
		}
	})
}
