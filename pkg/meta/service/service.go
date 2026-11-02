// Package service is the thin domain layer for the Meta Conversions API
// integration. It owns the rule that every server-side event carries
// action_source="website" and pre-hash PII, then delegates to the capi client.
package service

import (
	"context"
	"errors"

	"github.com/aseno/backend/pkg/meta/capi"
)

// UserData mirrors capi.UserData with raw (unhashed) PII. The capi client
// owns the SHA-256 normalization — service must NOT hash, or we'd double-hash.
type UserData struct {
	Email      string
	Phone      string
	ExternalID string
	FBP        string
	FBC        string
}

// TrackEventRequest is the contract the handler hands to the service.
// ClientIPAddress / ClientUserAgent are injected by the handler from the HTTP
// request — never trusted from the JSON body.
type TrackEventRequest struct {
	EventName       string
	EventID         string
	EventTime       int64
	EventSourceURL  string
	ClientIPAddress string
	ClientUserAgent string
	UserData        UserData
	CustomData      map[string]interface{}
}

// Client is the subset of *capi.Client that the service needs. Defined here
// so tests can swap a fake without dragging the HTTP layer in.
type Client interface {
	SendEvents(ctx context.Context, events []capi.Event) (*capi.Response, error)
}

// Service is the public API of this package.
type Service interface {
	TrackEvent(ctx context.Context, req TrackEventRequest) (*capi.Response, error)
}

type service struct {
	client Client
}

// NewService builds a Service wired to the given CAPI client.
func NewService(client Client) Service {
	return &service{client: client}
}

func (s *service) TrackEvent(ctx context.Context, req TrackEventRequest) (*capi.Response, error) {
	if req.EventName == "" {
		return nil, errors.New("meta: event_name is required")
	}
	if req.EventTime == 0 {
		return nil, errors.New("meta: event_time is required")
	}

	event := capi.Event{
		EventName:      req.EventName,
		EventTime:      req.EventTime,
		EventID:        req.EventID,
		ActionSource:   "website",
		EventSourceURL: req.EventSourceURL,
		UserData: capi.UserData{
			Email:           req.UserData.Email,
			Phone:           req.UserData.Phone,
			ExternalID:      req.UserData.ExternalID,
			ClientIPAddress: req.ClientIPAddress,
			ClientUserAgent: req.ClientUserAgent,
			FBP:             req.UserData.FBP,
			FBC:             req.UserData.FBC,
		},
		CustomData: req.CustomData,
	}

	return s.client.SendEvents(ctx, []capi.Event{event})
}
