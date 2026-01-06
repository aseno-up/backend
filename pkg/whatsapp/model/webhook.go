package model

import "time"

// WebhookEvent represents an incoming webhook event from Evolution API
type WebhookEvent struct {
	Event    string      `json:"event"`
	Instance string      `json:"instance"`
	Data     interface{} `json:"data"`
}

// WebhookEventType represents the type of webhook event
type WebhookEventType string

const (
	WebhookEventMessages       WebhookEventType = "messages.upsert"
	WebhookEventMessagesUpdate WebhookEventType = "messages.update"
	WebhookEventConnection     WebhookEventType = "connection.update"
	WebhookEventQRCode         WebhookEventType = "qrcode.updated"
)

// EvolutionWebhookPayload represents the Evolution API webhook payload
type EvolutionWebhookPayload struct {
	Event    string                 `json:"event"`
	Instance string                 `json:"instance"`
	Data     EvolutionWebhookData   `json:"data"`
}

// EvolutionWebhookData represents the data in Evolution webhook
type EvolutionWebhookData struct {
	Key               EvolutionMessageKey    `json:"key"`
	PushName          string                 `json:"pushName"`
	Message           EvolutionMessageContent `json:"message"`
	MessageType       string                 `json:"messageType"`
	MessageTimestamp  int64                  `json:"messageTimestamp"`
	Status            string                 `json:"status,omitempty"`
}

// EvolutionMessageKey represents the message key
type EvolutionMessageKey struct {
	RemoteJid string `json:"remoteJid"`
	FromMe    bool   `json:"fromMe"`
	ID        string `json:"id"`
}

// EvolutionMessageContent represents the message content
type EvolutionMessageContent struct {
	Conversation       string                      `json:"conversation,omitempty"`
	ExtendedTextMessage *EvolutionExtendedText     `json:"extendedTextMessage,omitempty"`
	ButtonsResponseMessage *EvolutionButtonResponse `json:"buttonsResponseMessage,omitempty"`
	ListResponseMessage    *EvolutionListResponse   `json:"listResponseMessage,omitempty"`
}

// EvolutionExtendedText represents extended text message
type EvolutionExtendedText struct {
	Text string `json:"text"`
}

// EvolutionButtonResponse represents a button response
type EvolutionButtonResponse struct {
	SelectedButtonID    string `json:"selectedButtonId"`
	SelectedDisplayText string `json:"selectedDisplayText"`
}

// EvolutionListResponse represents a list response
type EvolutionListResponse struct {
	SingleSelectReply struct {
		SelectedRowID string `json:"selectedRowId"`
	} `json:"singleSelectReply"`
}

// ParsedIncomingMessage represents a parsed incoming message
type ParsedIncomingMessage struct {
	Phone       string    `json:"phone"`
	SenderName  string    `json:"sender_name"`
	MessageID   string    `json:"message_id"`
	Content     string    `json:"content"`
	ButtonID    string    `json:"button_id,omitempty"`
	IsFromMe    bool      `json:"is_from_me"`
	Timestamp   time.Time `json:"timestamp"`
	Instance    string    `json:"instance"`
}

// MessageIntent represents the classified intent of an incoming message
type MessageIntent string

const (
	MessageIntentConfirm    MessageIntent = "confirm"
	MessageIntentCancel     MessageIntent = "cancel"
	MessageIntentReschedule MessageIntent = "reschedule"
	MessageIntentQuestion   MessageIntent = "question"
	MessageIntentUnknown    MessageIntent = "unknown"
)

// ClassifiedMessage represents a message with its classified intent
type ClassifiedMessage struct {
	ParsedIncomingMessage
	Intent     MessageIntent `json:"intent"`
	Confidence float64       `json:"confidence"`
}

// ConnectionStatus represents the WhatsApp connection status
type ConnectionStatus struct {
	Instance   string `json:"instance"`
	State      string `json:"state"`
	StatusCode int    `json:"status_code"`
}

// QRCodeData represents QR code data for connecting WhatsApp
type QRCodeData struct {
	Instance string `json:"instance"`
	QRCode   string `json:"qr_code"`
	Base64   string `json:"base64"`
}
