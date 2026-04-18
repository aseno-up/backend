package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aseno/backend/pkg/systemlog/model"
	"github.com/aseno/backend/pkg/systemlog/service"
)

// responseCapturer wraps gin.ResponseWriter so the middleware can inspect the
// body after the handler has already written it to the client.
type responseCapturer struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseCapturer) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// ErrorCapture sends every non-2xx response to the system_logs table so
// operators can triage failures without tailing the gin stdout log. The write
// is fire-and-forget inside the service (Log spawns a goroutine), so request
// latency is not impacted.
// fieldsToRedact are request payload keys we must not persist in system_logs.
// The match is case-insensitive and substring-based so variants (authorization,
// auth_token, etc.) are all caught.
var fieldsToRedact = []string{"password", "token", "secret", "authorization"}

// redactBody returns a safe preview of the request body. It preserves JSON
// keys so operators can still see which fields were sent while hiding values
// for sensitive keys, and it truncates to avoid bloating the logs.
func redactBody(body []byte) string {
	s := string(body)
	lower := strings.ToLower(s)
	for _, k := range fieldsToRedact {
		if strings.Contains(lower, k) {
			return "[redacted: contains sensitive field]"
		}
	}
	if len(s) > 2000 {
		s = s[:2000] + "…"
	}
	return s
}

func ErrorCapture(logSvc service.SystemLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Buffer the request body so both the handler and this middleware can
		// read it. gin/gorilla give us a one-shot io.ReadCloser, so we replace
		// it with a re-readable bytes.Reader.
		var reqBodyBytes []byte
		if c.Request.Body != nil && c.Request.ContentLength > 0 && c.Request.ContentLength < 1_000_000 {
			reqBodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
		}

		capturer := &responseCapturer{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = capturer

		start := time.Now()
		c.Next()
		duration := time.Since(start)

		status := c.Writer.Status()
		if status < http.StatusBadRequest {
			return
		}

		logStatus := model.LogStatusFailed
		if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			logStatus = model.LogStatusWarning
		}

		var clinicIDPtr *uuid.UUID
		if raw, ok := c.Get("clinic_id"); ok {
			if v, ok := raw.(uuid.UUID); ok {
				clinicIDPtr = &v
			}
		}

		errMsg := capturer.body.String()
		if len(errMsg) > 2000 {
			errMsg = errMsg[:2000]
		}

		payload := map[string]interface{}{
			"method":     c.Request.Method,
			"path":       c.FullPath(),
			"query":      c.Request.URL.RawQuery,
			"status":     status,
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}
		if len(reqBodyBytes) > 0 {
			payload["request_body"] = redactBody(reqBodyBytes)
		}
		if userID, ok := c.Get("user_id"); ok {
			payload["user_id"] = userID
		}
		if email, ok := c.Get("email"); ok {
			payload["user_email"] = email
		}
		if len(c.Errors) > 0 {
			payload["gin_errors"] = c.Errors.String()
		}

		_ = logSvc.Log(context.Background(), service.SystemLogEntry{
			ClinicID: clinicIDPtr,
			Service:  "http",
			Event:    c.Request.Method + " " + c.FullPath(),
			Status:   logStatus,
			Payload:  payload,
			Error:    errMsg,
			Duration: int(duration / time.Millisecond),
		})
	}
}
