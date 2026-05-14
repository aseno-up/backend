package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	meta_service "github.com/aseno/backend/pkg/meta/service"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// stubService records the request it gets and lets the test choose the return.
type stubService struct {
	got TrackedReq
	err error
}

// TrackedReq is a copy of meta_service.TrackEventRequest captured on the way
// in. We keep an alias here so handler tests don't have to drag the full
// service type into their assertions.
type TrackedReq = meta_service.TrackEventRequest

func (s *stubService) TrackEvent(_ context.Context, req meta_service.TrackEventRequest) error {
	s.got = req
	return s.err
}

func performRequest(h *MetaHandler, body string, headers map[string]string) *httptest.ResponseRecorder {
	router := gin.New()
	router.POST("/api/v1/meta/events", h.TrackEvent)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/meta/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHandler_TrackEvent_AcceptsValidPayload(t *testing.T) {
	svc := &stubService{}
	h := NewMetaHandler(svc)

	body := `{
		"event_id":"evt-abc",
		"event_name":"Lead",
		"event_source_url":"https://aseno.app/planos?plan=professional",
		"user_data":{
			"email":"foo@bar.com",
			"phone":"+55 11 99999-0000",
			"external_id":"user-1",
			"fbp":"fb.1.x.y",
			"fbc":"fb.1.x.z"
		},
		"custom_data":{"currency":"BRL","value":199,"content_name":"professional"}
	}`

	rec := performRequest(h, body, map[string]string{
		"User-Agent":      "Mozilla/5.0",
		"X-Forwarded-For": "203.0.113.42",
	})

	assert.Equal(t, http.StatusAccepted, rec.Code, "body=%s", rec.Body.String())

	got := svc.got
	assert.Equal(t, "Lead", got.EventName)
	assert.Equal(t, "evt-abc", got.EventID)
	assert.Greater(t, got.EventTime, int64(0), "event_time must be filled server-side (unix seconds)")
	assert.Equal(t, "Mozilla/5.0", got.ClientUserAgent, "UA must come from header")
	// gin's c.ClientIP() reads X-Forwarded-For only when trusted proxies are
	// configured; the default trusts none, so we either get RemoteAddr or the
	// XFF value. Either way, IP must be non-empty.
	assert.NotEmpty(t, got.ClientIPAddress, "IP must be filled from request")

	assert.Equal(t, "foo@bar.com", got.UserData.Email)
	assert.Equal(t, "+55 11 99999-0000", got.UserData.Phone)
	assert.Equal(t, "user-1", got.UserData.ExternalID)
	assert.Equal(t, "fb.1.x.y", got.UserData.FBP)
	assert.Equal(t, "fb.1.x.z", got.UserData.FBC)

	assert.Equal(t, "BRL", got.CustomData["currency"])
	assert.EqualValues(t, 199, got.CustomData["value"])
}

func TestHandler_TrackEvent_RejectsMissingEventName(t *testing.T) {
	svc := &stubService{}
	h := NewMetaHandler(svc)

	rec := performRequest(h, `{"event_id":"e1"}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, svc.got.EventName, "service must not be called for invalid body")
}

func TestHandler_TrackEvent_RejectsMalformedJSON(t *testing.T) {
	svc := &stubService{}
	h := NewMetaHandler(svc)

	rec := performRequest(h, `not json`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_TrackEvent_ReturnsBadGatewayOnUpstreamError(t *testing.T) {
	svc := &stubService{err: errors.New("capi: 500 internal error")}
	h := NewMetaHandler(svc)

	rec := performRequest(h, `{"event_id":"e1","event_name":"Lead"}`, nil)
	// Upstream Meta failures shouldn't be hidden as 5xx-from-us; surface as
	// 502 so callers can ignore it and rely on the browser pixel.
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestHandler_TrackEvent_DefaultsEventTimeWhenMissing(t *testing.T) {
	svc := &stubService{}
	h := NewMetaHandler(svc)

	// Caller omitted event_time → handler fills with now().
	rec := performRequest(h, `{"event_id":"e1","event_name":"PageView"}`, nil)
	require.Equal(t, http.StatusAccepted, rec.Code, "body=%s", rec.Body.String())
	assert.Greater(t, svc.got.EventTime, int64(1700000000), "must default to current unix time")
}

func TestHandler_TrackEvent_ResponseShape(t *testing.T) {
	svc := &stubService{}
	h := NewMetaHandler(svc)

	rec := performRequest(h, `{"event_id":"e1","event_name":"Lead"}`, nil)
	require.Equal(t, http.StatusAccepted, rec.Code)
	var out map[string]interface{}
	require.NoError(t, json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&out))
	assert.Equal(t, true, out["ok"])
}
