package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/pkg/meta/capi"
)

// recordingClient captures the Event(s) handed to the CAPI client so we can
// assert the service translation layer end-to-end. Returning an error from
// SendEvents lets us also exercise the failure path.
type recordingClient struct {
	got    []capi.Event
	resp   *capi.Response
	sendOk bool
	err    error
}

func (r *recordingClient) SendEvents(_ context.Context, events []capi.Event) (*capi.Response, error) {
	r.got = events
	if !r.sendOk {
		return nil, r.err
	}
	return r.resp, nil
}

func TestService_TrackEvent_TranslatesAndForwards(t *testing.T) {
	client := &recordingClient{sendOk: true, resp: &capi.Response{EventsReceived: 1, FBTraceID: "tr-1"}}
	svc := NewService(client)

	err := svc.TrackEvent(context.Background(), TrackEventRequest{
		EventName:       "Lead",
		EventID:         "evt-1",
		EventTime:       1715800000,
		EventSourceURL:  "https://aseno.app/planos",
		ClientIPAddress: "203.0.113.42",
		ClientUserAgent: "Mozilla/5.0",
		UserData: UserData{
			Email:      "foo@bar.com",
			Phone:      "+55 11 99999-0000",
			ExternalID: "uuid-1",
			FBP:        "fb.1.x.y",
			FBC:        "fb.1.x.z",
		},
		CustomData: map[string]interface{}{"currency": "BRL", "value": 199.0},
	})
	require.NoError(t, err)
	require.Len(t, client.got, 1)

	ev := client.got[0]
	assert.Equal(t, "Lead", ev.EventName)
	assert.EqualValues(t, 1715800000, ev.EventTime)
	assert.Equal(t, "evt-1", ev.EventID)
	assert.Equal(t, "website", ev.ActionSource, "service must default action_source to 'website'")
	assert.Equal(t, "https://aseno.app/planos", ev.EventSourceURL)

	// Service must pass raw PII through; hashing is the client's job.
	assert.Equal(t, "foo@bar.com", ev.UserData.Email)
	assert.Equal(t, "+55 11 99999-0000", ev.UserData.Phone)
	assert.Equal(t, "uuid-1", ev.UserData.ExternalID)
	assert.Equal(t, "203.0.113.42", ev.UserData.ClientIPAddress, "IP from request must reach the client")
	assert.Equal(t, "Mozilla/5.0", ev.UserData.ClientUserAgent, "UA from request must reach the client")
	assert.Equal(t, "fb.1.x.y", ev.UserData.FBP)
	assert.Equal(t, "fb.1.x.z", ev.UserData.FBC)

	assert.Equal(t, "BRL", ev.CustomData["currency"])
	assert.EqualValues(t, 199.0, ev.CustomData["value"])
}

func TestService_TrackEvent_RejectsMissingFields(t *testing.T) {
	client := &recordingClient{sendOk: true, resp: &capi.Response{}}
	svc := NewService(client)

	cases := []struct {
		name string
		req  TrackEventRequest
	}{
		{
			name: "missing event_name",
			req:  TrackEventRequest{EventTime: 1, ClientIPAddress: "1.1.1.1"},
		},
		{
			name: "missing event_time",
			req:  TrackEventRequest{EventName: "Lead", ClientIPAddress: "1.1.1.1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.TrackEvent(context.Background(), tc.req)
			require.Error(t, err)
			assert.Nil(t, client.got, "client must not be called for invalid input")
			client.got = nil
		})
	}
}

func TestService_TrackEvent_SurfacesClientError(t *testing.T) {
	client := &recordingClient{sendOk: false, err: errors.New("capi: 502 Bad Gateway")}
	svc := NewService(client)

	err := svc.TrackEvent(context.Background(), TrackEventRequest{
		EventName: "Lead",
		EventTime: 1715800000,
		EventID:   "evt-x",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}
