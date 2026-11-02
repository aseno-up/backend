package capi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// metaCAPIDebug toggles a one-shot log of the outgoing payload (access_token
// redacted) so we can confirm the wire format Meta is receiving. Enable by
// setting META_CAPI_DEBUG=1.
var metaCAPIDebug = os.Getenv("META_CAPI_DEBUG") == "1"

const defaultBaseURL = "https://graph.facebook.com"

// Config carries the Meta Conversions API credentials and tuning.
type Config struct {
	// BaseURL overrides graph.facebook.com (for tests). Empty → production.
	BaseURL string
	// PixelID is the Meta Pixel / dataset ID (same value used by fbq init).
	PixelID string
	// AccessToken is the system-user / dataset access token from Events Manager.
	AccessToken string
	// APIVersion e.g. "v21.0". Empty defaults to v21.0.
	APIVersion string
	// TestEventCode, when set, routes events to "Test Events" in Events Manager
	// without affecting real attribution. Leave empty in production.
	TestEventCode string
	// HTTPClient lets callers inject a custom client (timeouts, transport).
	HTTPClient *http.Client
}

// Client posts events to Meta CAPI.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// NewClient returns a Client. It does not validate credentials — SendEvents
// surfaces config errors at call time so a missing token never silently no-ops.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = "v21.0"
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{cfg: cfg, httpClient: hc}
}

// UserData captures the identifiers Meta uses to match a server event back
// to a person. PII fields (Email, Phone, ExternalID) are hashed inside
// SendEvents — callers pass plain values. Non-PII fields (IP, UA, fbp, fbc)
// must NOT be hashed per Meta's spec.
type UserData struct {
	Email           string
	Phone           string
	ExternalID      string
	ClientIPAddress string
	ClientUserAgent string
	FBP             string
	FBC             string
}

// Event is one server-side conversion. EventID must match the browser pixel's
// eventID for the same user action so Meta can deduplicate.
type Event struct {
	EventName      string
	EventTime      int64
	EventID        string
	ActionSource   string // "website" for web traffic
	EventSourceURL string
	UserData       UserData
	CustomData     map[string]interface{}
}

// Response is the CAPI success envelope.
type Response struct {
	EventsReceived int    `json:"events_received"`
	FBTraceID      string `json:"fbtrace_id"`
}

// metaErrorEnvelope is the shape graph.facebook.com returns on failure.
type metaErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// SendEvents posts a batch. Returns an error if Meta responds non-2xx or if
// the client is misconfigured.
func (c *Client) SendEvents(ctx context.Context, events []Event) (*Response, error) {
	if c.cfg.PixelID == "" {
		return nil, errors.New("capi: PixelID is required")
	}
	if c.cfg.AccessToken == "" {
		return nil, errors.New("capi: AccessToken is required")
	}
	if len(events) == 0 {
		return nil, errors.New("capi: at least one event is required")
	}

	payload := map[string]interface{}{
		"data":         buildDataArray(events),
		"access_token": c.cfg.AccessToken,
	}
	if c.cfg.TestEventCode != "" {
		payload["test_event_code"] = c.cfg.TestEventCode
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("capi: marshal payload: %w", err)
	}

	if metaCAPIDebug {
		// Re-marshal with access_token stripped so we never log it.
		safe := make(map[string]interface{}, len(payload))
		for k, v := range payload {
			if k == "access_token" {
				continue
			}
			safe[k] = v
		}
		if dbg, e := json.Marshal(safe); e == nil {
			log.Printf("[meta-capi] outgoing payload: %s", string(dbg))
		}
	}

	url := fmt.Sprintf("%s/%s/%s/events",
		strings.TrimRight(c.cfg.BaseURL, "/"),
		c.cfg.APIVersion,
		c.cfg.PixelID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("capi: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("capi: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("capi: read response: %w", err)
	}

	if metaCAPIDebug {
		log.Printf("[meta-capi] response %d body: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		var env metaErrorEnvelope
		if json.Unmarshal(respBody, &env) == nil && env.Error.Message != "" {
			msg = env.Error.Message
		}
		return nil, fmt.Errorf("capi: %d %s", resp.StatusCode, msg)
	}

	var out Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("capi: decode response: %w", err)
	}
	return &out, nil
}

func buildDataArray(events []Event) []map[string]interface{} {
	data := make([]map[string]interface{}, 0, len(events))
	for _, e := range events {
		entry := map[string]interface{}{
			"event_name":    e.EventName,
			"event_time":    e.EventTime,
			"action_source": e.ActionSource,
			"user_data":     buildUserData(e.UserData),
		}
		if e.EventID != "" {
			entry["event_id"] = e.EventID
		}
		if e.EventSourceURL != "" {
			entry["event_source_url"] = e.EventSourceURL
		}
		if len(e.CustomData) > 0 {
			entry["custom_data"] = e.CustomData
		}
		data = append(data, entry)
	}
	return data
}

func buildUserData(u UserData) map[string]interface{} {
	out := map[string]interface{}{}
	if h := HashEmail(u.Email); h != "" {
		out["em"] = []string{h}
	}
	if h := HashPhone(u.Phone); h != "" {
		out["ph"] = []string{h}
	}
	if h := HashSHA256Hex(strings.TrimSpace(u.ExternalID)); h != "" {
		out["external_id"] = []string{h}
	}
	if u.ClientIPAddress != "" {
		out["client_ip_address"] = u.ClientIPAddress
	}
	if u.ClientUserAgent != "" {
		out["client_user_agent"] = u.ClientUserAgent
	}
	if u.FBP != "" {
		out["fbp"] = u.FBP
	}
	if u.FBC != "" {
		out["fbc"] = u.FBC
	}
	return out
}
