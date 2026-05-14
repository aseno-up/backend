package capi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundtripPayload mirrors the wire format the Meta CAPI expects. We decode
// the request body into this and assert on each field individually so a
// regression in any one of them shows up clearly.
type roundtripPayload struct {
	Data          []map[string]interface{} `json:"data"`
	AccessToken   string                   `json:"access_token"`
	TestEventCode string                   `json:"test_event_code,omitempty"`
}

func TestClient_SendEvents_PostsExpectedPayload(t *testing.T) {
	var captured roundtripPayload
	var capturedPath string
	var capturedMethod string
	var capturedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &captured))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"events_received":1,"messages":[],"fbtrace_id":"trace-1"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:     server.URL,
		PixelID:     "1268822281463738",
		AccessToken: "EAAG-test-token",
		APIVersion:  "v21.0",
	})

	event := Event{
		EventName:      "Lead",
		EventTime:      1715800000,
		EventID:        "evt-abc-123",
		ActionSource:   "website",
		EventSourceURL: "https://aseno.app/planos?plan=professional",
		UserData: UserData{
			Email:           "Foo.Bar@Example.com",
			Phone:           "+55 11 99999-0000",
			ClientIPAddress: "203.0.113.42",
			ClientUserAgent: "Mozilla/5.0",
			FBP:             "fb.1.1715800000.1234567890",
			FBC:             "fb.1.1715800000.AbCd",
			ExternalID:      "user-uuid-1",
		},
		CustomData: map[string]interface{}{
			"currency":     "BRL",
			"value":        199.0,
			"content_name": "professional",
		},
	}

	resp, err := client.SendEvents(context.Background(), []Event{event})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.EventsReceived)
	assert.Equal(t, "trace-1", resp.FBTraceID)

	// Transport-level assertions.
	assert.Equal(t, http.MethodPost, capturedMethod)
	assert.Equal(t, "/v21.0/1268822281463738/events", capturedPath)
	assert.True(t, strings.HasPrefix(capturedContentType, "application/json"),
		"Content-Type must be application/json, got %q", capturedContentType)

	// Auth and dedup envelope.
	assert.Equal(t, "EAAG-test-token", captured.AccessToken)
	assert.Empty(t, captured.TestEventCode, "test_event_code should be omitted when unset")
	require.Len(t, captured.Data, 1)

	got := captured.Data[0]
	assert.Equal(t, "Lead", got["event_name"])
	assert.EqualValues(t, 1715800000, got["event_time"])
	assert.Equal(t, "evt-abc-123", got["event_id"], "event_id required for dedup with browser pixel")
	assert.Equal(t, "website", got["action_source"])
	assert.Equal(t, "https://aseno.app/planos?plan=professional", got["event_source_url"])

	// user_data: PII fields must be hashed (array of sha256 hex), non-PII raw.
	ud, ok := got["user_data"].(map[string]interface{})
	require.True(t, ok, "user_data must be an object")

	emArr, ok := ud["em"].([]interface{})
	require.True(t, ok, "em must be an array of hashes")
	require.Len(t, emArr, 1)
	assert.Equal(t, HashEmail("Foo.Bar@Example.com"), emArr[0])

	phArr, ok := ud["ph"].([]interface{})
	require.True(t, ok, "ph must be an array of hashes")
	require.Len(t, phArr, 1)
	assert.Equal(t, HashPhone("+55 11 99999-0000"), phArr[0])

	extArr, ok := ud["external_id"].([]interface{})
	require.True(t, ok, "external_id must be an array")
	require.Len(t, extArr, 1)
	assert.Equal(t, HashSHA256Hex("user-uuid-1"), extArr[0])

	assert.Equal(t, "203.0.113.42", ud["client_ip_address"], "IP must NOT be hashed")
	assert.Equal(t, "Mozilla/5.0", ud["client_user_agent"], "UA must NOT be hashed")
	assert.Equal(t, "fb.1.1715800000.1234567890", ud["fbp"])
	assert.Equal(t, "fb.1.1715800000.AbCd", ud["fbc"])

	// custom_data forwarded verbatim.
	cd, ok := got["custom_data"].(map[string]interface{})
	require.True(t, ok, "custom_data must be an object")
	assert.Equal(t, "BRL", cd["currency"])
	assert.EqualValues(t, 199.0, cd["value"])
	assert.Equal(t, "professional", cd["content_name"])
}

func TestClient_SendEvents_OmitsEmptyUserDataFields(t *testing.T) {
	// When the caller has no email/phone/external_id (e.g. anonymous PageView
	// from a visitor with only fbp/IP/UA), the hashed arrays must be omitted —
	// sending hashes of empty strings poisons match quality.
	var captured roundtripPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"events_received":1}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:     server.URL,
		PixelID:     "1268822281463738",
		AccessToken: "tk",
		APIVersion:  "v21.0",
	})

	_, err := client.SendEvents(context.Background(), []Event{{
		EventName:    "PageView",
		EventTime:    1715800000,
		EventID:      "pv-1",
		ActionSource: "website",
		UserData: UserData{
			ClientIPAddress: "203.0.113.42",
			ClientUserAgent: "Mozilla/5.0",
			FBP:             "fb.1.x.y",
		},
	}})
	require.NoError(t, err)
	require.Len(t, captured.Data, 1)

	ud, ok := captured.Data[0]["user_data"].(map[string]interface{})
	require.True(t, ok)
	_, hasEm := ud["em"]
	_, hasPh := ud["ph"]
	_, hasExt := ud["external_id"]
	assert.False(t, hasEm, "em must be omitted when email is empty")
	assert.False(t, hasPh, "ph must be omitted when phone is empty")
	assert.False(t, hasExt, "external_id must be omitted when empty")
}

func TestClient_SendEvents_IncludesTestEventCodeWhenSet(t *testing.T) {
	var captured roundtripPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"events_received":1}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:       server.URL,
		PixelID:       "1268822281463738",
		AccessToken:   "tk",
		APIVersion:    "v21.0",
		TestEventCode: "TEST12345",
	})

	_, err := client.SendEvents(context.Background(), []Event{{
		EventName:    "Lead",
		EventTime:    1715800000,
		EventID:      "evt-1",
		ActionSource: "website",
	}})
	require.NoError(t, err)
	assert.Equal(t, "TEST12345", captured.TestEventCode)
}

func TestClient_SendEvents_ReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid event_time","code":100}}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:     server.URL,
		PixelID:     "px",
		AccessToken: "tk",
		APIVersion:  "v21.0",
	})

	resp, err := client.SendEvents(context.Background(), []Event{{
		EventName:    "Lead",
		EventTime:    1,
		ActionSource: "website",
	}})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "400", "error must surface upstream status")
	assert.Contains(t, err.Error(), "Invalid event_time", "error must surface upstream message")
}

func TestClient_SendEvents_RejectsEmptyBatch(t *testing.T) {
	// Calling Meta with an empty data array is wasted RT and triggers a 400.
	// Reject locally so callers get a clear error.
	client := NewClient(Config{
		BaseURL:     "http://unused",
		PixelID:     "px",
		AccessToken: "tk",
		APIVersion:  "v21.0",
	})
	resp, err := client.SendEvents(context.Background(), nil)
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestClient_SendEvents_RequiresConfig(t *testing.T) {
	// Missing PixelID or AccessToken means the integration is not configured;
	// we must not silently hit the network with a half-built URL.
	cases := []struct {
		name string
		cfg  Config
	}{
		{name: "no pixel id", cfg: Config{AccessToken: "tk", APIVersion: "v21.0"}},
		{name: "no access token", cfg: Config{PixelID: "px", APIVersion: "v21.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(tc.cfg)
			resp, err := client.SendEvents(context.Background(), []Event{{
				EventName: "Lead", EventTime: 1, ActionSource: "website",
			}})
			require.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}
