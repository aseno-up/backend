package gupshup

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aseno/backend/pkg/whatsapp/model"
)

// WebhookPayload represents the Gupshup inbound message payload structure
type WebhookPayload struct {
	App       string `json:"app"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"` // "message" or "message-event"
	Payload   struct {
		ID      string `json:"id"`
		Source  string `json:"source"`
		Type    string `json:"type"` // "text", "audio", "button_reply", etc.
		Payload struct {
			Text string `json:"text,omitempty"`
			URL  string `json:"url,omitempty"`
			ID   string `json:"id,omitempty"` // For interactive buttons
		} `json:"payload"`
		Sender struct {
			Phone string `json:"phone"`
			Name  string `json:"name"`
		} `json:"sender"`
	} `json:"payload"`
}

// EventPayload represents delivery/read receipts
type EventPayload struct {
	App       string `json:"app"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"` // "message-event"
	Payload   struct {
		ID          string `json:"id"`
		Type        string `json:"type"` // "enqueued", "sent", "delivered", "read", "failed"
		Destination string `json:"destination"`
	} `json:"payload"`
}

// ParseWebhookPayload parses an incoming Gupshup webhook payload into the standard generic ParsedIncomingMessage format
func ParseWebhookPayload(body []byte) (*model.ParsedIncomingMessage, error) {
	// First determine the type of webhook event
	var baseEvent struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &baseEvent); err != nil {
		return nil, fmt.Errorf("failed to parse basic gupshup event: %w", err)
	}

	if baseEvent.Type == "message-event" {
		// Log message events like sent, delivered, read status but don't process as incoming messages
		// We could potentially update message status in DB here if we implement message-event parsing
		return nil, nil // Return nil because this is not an incoming message from the user
	}

	if baseEvent.Type != "message" {
		// We only process 'message' type for incoming conversational data
		return nil, nil
	}

	var msg WebhookPayload
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse gupshup message payload: %w", err)
	}

	// Normalize phone number (strip + or anything, we expect clean digits e.g. 5511999990000)
	phone := msg.Payload.Sender.Phone
	phone = strings.ReplaceAll(phone, "+", "")

	result := &model.ParsedIncomingMessage{
		Instance:   msg.App, // Using the App name as instance identifier
		MessageID:  msg.Payload.ID,
		Phone:      phone,
		SenderName: msg.Payload.Sender.Name,
		Timestamp:  time.UnixMilli(msg.Timestamp),
		IsFromMe:   false, // Gupshup Sandbox inbound messages are from users
	}

	switch msg.Payload.Type {
	case "text":
		result.Content = msg.Payload.Payload.Text
	case "audio":
		result.IsAudio = true
		result.AudioURL = msg.Payload.Payload.URL
	case "button_reply", "list_reply": // interactive replies
		// The replied text or ID
		if msg.Payload.Payload.Text != "" {
			result.Content = msg.Payload.Payload.Text
		} else {
			result.Content = msg.Payload.Payload.ID
			result.ButtonID = msg.Payload.Payload.ID
		}
	default:
		// Unsupported or other types like image, document, we can store loosely
		result.Content = fmt.Sprintf("[Received %s message]", msg.Payload.Type)
	}

	return result, nil
}
