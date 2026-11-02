// Package handler exposes POST /api/v1/meta/events — the server-side mirror
// that the browser pixel calls in parallel with fbq('track', ...). The same
// event_id on both sides lets Meta deduplicate.
package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	meta_service "github.com/aseno/backend/pkg/meta/service"
)

// MetaHandler holds the HTTP handlers.
type MetaHandler struct {
	svc meta_service.Service
}

// NewMetaHandler wires the handler to its service.
func NewMetaHandler(svc meta_service.Service) *MetaHandler {
	return &MetaHandler{svc: svc}
}

// RegisterRoutes wires the public CAPI mirror endpoint into a Gin router.
// No auth — the browser calls this from the landing page.
func (h *MetaHandler) RegisterRoutes(r gin.IRouter) {
	r.POST("/meta/events", h.TrackEvent)
}

// trackEventRequest is the JSON the browser sends. event_time and the IP/UA
// are NOT trusted from the body — handler fills them.
type trackEventRequest struct {
	EventID        string                 `json:"event_id"`
	EventName      string                 `json:"event_name" binding:"required"`
	EventSourceURL string                 `json:"event_source_url"`
	UserData       trackEventUserData     `json:"user_data"`
	CustomData     map[string]interface{} `json:"custom_data"`
}

type trackEventUserData struct {
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	ExternalID string `json:"external_id"`
	FBP        string `json:"fbp"`
	FBC        string `json:"fbc"`
}

// TrackEvent forwards a browser-side event to Meta CAPI via the service.
//
// Status codes:
//   - 202 Accepted     event was forwarded successfully
//   - 400 Bad Request  body is malformed or missing event_name
//   - 502 Bad Gateway  Meta CAPI returned a non-2xx; safe to ignore on the
//     frontend since the browser pixel will still count
func (h *MetaHandler) TrackEvent(c *gin.Context) {
	var req trackEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	svcReq := meta_service.TrackEventRequest{
		EventName:       req.EventName,
		EventID:         req.EventID,
		EventTime:       time.Now().Unix(),
		EventSourceURL:  req.EventSourceURL,
		ClientIPAddress: c.ClientIP(),
		ClientUserAgent: c.GetHeader("User-Agent"),
		UserData: meta_service.UserData{
			Email:      req.UserData.Email,
			Phone:      req.UserData.Phone,
			ExternalID: req.UserData.ExternalID,
			FBP:        req.UserData.FBP,
			FBC:        req.UserData.FBC,
		},
		CustomData: req.CustomData,
	}

	resp, err := h.svc.TrackEvent(c.Request.Context(), svcReq)
	if err != nil {
		// Don't crash analytics if Meta is misbehaving — log + 502 so frontend
		// can swallow the response without retrying.
		log.Printf("[meta-capi] forward failed event=%s id=%s err=%v", req.EventName, req.EventID, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "meta capi upstream"})
		return
	}
	// Log success so we can correlate the request with Meta's fbtrace_id when
	// debugging missing events in the Test Events / Overview panels.
	if resp != nil {
		log.Printf("[meta-capi] ok event=%s id=%s events_received=%d fbtrace=%s",
			req.EventName, req.EventID, resp.EventsReceived, resp.FBTraceID)
	}

	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}
