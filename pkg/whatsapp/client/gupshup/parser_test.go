package gupshup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWebhookPayload_Message(t *testing.T) {
	payload := []byte(`{
		"app": "TestApp",
		"timestamp": 1580227766370,
		"version": 2,
		"type": "message",
		"payload": {
		  "id": "ABEGkYaYVSEEAhAL3SLAWwHKeKrt6s3FKO0c",
		  "source": "918x98xx21x4",
		  "type": "text",
		  "payload": {
			"text": "hi"
		  },
		  "sender": {
			"phone": "918x98xx21x4",
			"name": "John Doe"
		  }
		}
	  }`)

	parsed, err := ParseWebhookPayload(payload)
	require.NoError(t, err)
	require.NotNil(t, parsed)

	assert.Equal(t, "TestApp", parsed.Instance)
	assert.Equal(t, "ABEGkYaYVSEEAhAL3SLAWwHKeKrt6s3FKO0c", parsed.MessageID)
	assert.Equal(t, "918x98xx21x4", parsed.Phone)
	assert.Equal(t, "John Doe", parsed.SenderName)
	assert.Equal(t, false, parsed.IsAudio)
	assert.Equal(t, "hi", parsed.Content)
}

func TestParseWebhookPayload_MessageEvent(t *testing.T) {
	payload := []byte(`{
		"app": "TestApp",
		"timestamp": 1580227766370,
		"version": 2,
		"type": "message-event",
		"payload": {
		  "id": "ABEGkYaYVSEEAhAL3SLAWwHKeKrt6s3FKO0c",
		  "type": "delivered",
		  "destination": "918x98xx21x4"
		}
	  }`)

	parsed, err := ParseWebhookPayload(payload)
	require.NoError(t, err)
	assert.Nil(t, parsed, "Message events should return nil parsed message")
}
