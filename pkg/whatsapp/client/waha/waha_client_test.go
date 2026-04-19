package waha

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseWebhookPayload_DirectMessage verifies that a 1-to-1 WAHA message
// (From suffix @c.us) parses with IsGroup=false so it is processed normally.
func TestParseWebhookPayload_DirectMessage(t *testing.T) {
	payload := map[string]any{
		"event":   "message",
		"session": "session-1",
		"payload": map[string]any{
			"id":        "msg-1",
			"timestamp": int64(1000000001),
			"from":      "5511999990000@c.us",
			"body":      "olá",
			"fromMe":    false,
		},
	}
	body, _ := json.Marshal(payload)

	parsed, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "5511999990000", parsed.Phone)
	assert.False(t, parsed.IsGroup)
}

// TestParseWebhookPayload_GroupMessage verifies that WAHA group payloads
// (From suffix @g.us) flag the message as group so the handler can skip.
func TestParseWebhookPayload_GroupMessage(t *testing.T) {
	payload := map[string]any{
		"event":   "message",
		"session": "session-1",
		"payload": map[string]any{
			"id":        "msg-2",
			"timestamp": int64(1000000002),
			"from":      "120363040123456789@g.us",
			"body":      "mensagem de grupo",
			"fromMe":    false,
		},
	}
	body, _ := json.Marshal(payload)

	parsed, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.True(t, parsed.IsGroup, "messages with @g.us JID must be flagged as group")
}
