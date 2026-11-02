package evolution

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseWebhookPayload_DirectMessage verifies the baseline: 1-to-1
// conversations (JID suffix @s.whatsapp.net) are parsed with IsGroup=false so
// the handler processes them normally.
func TestParseWebhookPayload_DirectMessage(t *testing.T) {
	payload := map[string]any{
		"event":    "messages.upsert",
		"instance": "clinic-1",
		"data": map[string]any{
			"key": map[string]any{
				"remoteJid": "5511999990000@s.whatsapp.net",
				"fromMe":    false,
				"id":        "MSG-1",
			},
			"message":          map[string]any{"conversation": "quero agendar"},
			"messageTimestamp": 1000000001,
		},
	}
	body, _ := json.Marshal(payload)

	parsed, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "5511999990000", parsed.Phone)
	assert.False(t, parsed.IsGroup, "direct messages must not be flagged as group")
}

// TestParseWebhookPayload_GroupMessage verifies that messages from group
// chats (JID suffix @g.us) set IsGroup=true so the webhook handler can skip
// them. The downstream service is not responsible for group detection.
func TestParseWebhookPayload_GroupMessage(t *testing.T) {
	payload := map[string]any{
		"event":    "messages.upsert",
		"instance": "clinic-1",
		"data": map[string]any{
			"key": map[string]any{
				"remoteJid": "120363040123456789@g.us",
				"fromMe":    false,
				"id":        "MSG-2",
			},
			"message":          map[string]any{"conversation": "mensagem de grupo"},
			"messageTimestamp": 1000000002,
		},
	}
	body, _ := json.Marshal(payload)

	parsed, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.True(t, parsed.IsGroup, "messages with @g.us JID must be flagged as group")
}
