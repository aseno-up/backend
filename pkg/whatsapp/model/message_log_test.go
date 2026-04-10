package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageLogPayload_Value_ReturnsString(t *testing.T) {
	payload := MessageLogPayload{"key": "value", "number": float64(42)}

	val, err := payload.Value()
	require.NoError(t, err)

	// Must return string (not []byte) so pgx sends it as text/json, not bytea
	str, ok := val.(string)
	assert.True(t, ok, "Value() must return string, got %T", val)
	assert.Contains(t, str, `"key"`)
	assert.Contains(t, str, `"value"`)
}

func TestMessageLogPayload_Value_NilReturnsNil(t *testing.T) {
	var payload MessageLogPayload

	val, err := payload.Value()
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestMessageLogPayload_Scan_Bytes(t *testing.T) {
	var payload MessageLogPayload
	err := payload.Scan([]byte(`{"foo":"bar"}`))
	require.NoError(t, err)
	assert.Equal(t, "bar", payload["foo"])
}

func TestMessageLogPayload_Scan_String(t *testing.T) {
	var payload MessageLogPayload
	err := payload.Scan(`{"foo":"bar"}`)
	require.NoError(t, err)
	assert.Equal(t, "bar", payload["foo"])
}

func TestMessageLogPayload_Scan_Nil(t *testing.T) {
	payload := MessageLogPayload{"existing": "data"}
	err := payload.Scan(nil)
	require.NoError(t, err)
	assert.Nil(t, payload)
}

func TestMessageLog_TableName(t *testing.T) {
	log := MessageLog{}
	assert.Equal(t, "message_logs", log.TableName())
}
