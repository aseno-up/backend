package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aseno/backend/pkg/calendar/model"
)

const (
	googleOAuthURL   = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL   = "https://oauth2.googleapis.com/token"
	googleCalendarAPI = "https://www.googleapis.com/calendar/v3"
)

// Config holds the Google Calendar client configuration
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Client implements the Google Calendar API client
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient creates a new Google Calendar client
func NewClient(config Config) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetAuthURL returns the Google OAuth authorization URL
func (c *Client) GetAuthURL(state string) string {
	params := url.Values{
		"client_id":     {c.config.ClientID},
		"redirect_uri":  {c.config.RedirectURL},
		"response_type": {"code"},
		"scope":         {"https://www.googleapis.com/auth/calendar https://www.googleapis.com/auth/userinfo.email"},
		"access_type":   {"offline"},
		"prompt":        {"consent select_account"},
		"state":         {state},
	}
	return googleOAuthURL + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens
func (c *Client) ExchangeCode(ctx context.Context, code string) (*model.GoogleOAuthTokens, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"redirect_uri":  {c.config.RedirectURL},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &model.GoogleOAuthTokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

// RefreshAccessToken refreshes an expired access token
func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) (*model.GoogleOAuthTokens, error) {
	data := url.Values{
		"refresh_token": {refreshToken},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &model.GoogleOAuthTokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: refreshToken, // Keep the same refresh token
		TokenType:    tokenResp.TokenType,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

// GetUserEmail gets the email of the authenticated user
func (c *Client) GetUserEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("user info request failed: %s", string(body))
	}

	var userInfo struct {
		Email string `json:"email"`
	}

	if err := json.Unmarshal(body, &userInfo); err != nil {
		return "", fmt.Errorf("failed to parse user info: %w", err)
	}

	return userInfo.Email, nil
}

// ListCalendars lists all calendars for the user
func (c *Client) ListCalendars(ctx context.Context, accessToken string) ([]model.GoogleCalendar, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleCalendarAPI+"/users/me/calendarList", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calendar list request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendar list request failed: %s", string(body))
	}

	var listResp model.CalendarListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse calendar list: %w", err)
	}

	return listResp.Items, nil
}

// CreateEvent creates a new event in the specified calendar
func (c *Client) CreateEvent(ctx context.Context, accessToken, calendarID string, event *model.GoogleEvent) (*model.GoogleEvent, error) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	url := fmt.Sprintf("%s/calendars/%s/events", googleCalendarAPI, url.PathEscape(calendarID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(eventJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create event request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create event failed: %s", string(body))
	}

	var createdEvent model.GoogleEvent
	if err := json.Unmarshal(body, &createdEvent); err != nil {
		return nil, fmt.Errorf("failed to parse created event: %w", err)
	}

	return &createdEvent, nil
}

// UpdateEvent updates an existing event
func (c *Client) UpdateEvent(ctx context.Context, accessToken, calendarID, eventID string, event *model.GoogleEvent) (*model.GoogleEvent, error) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	url := fmt.Sprintf("%s/calendars/%s/events/%s", googleCalendarAPI, url.PathEscape(calendarID), url.PathEscape(eventID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(eventJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update event request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update event failed: %s", string(body))
	}

	var updatedEvent model.GoogleEvent
	if err := json.Unmarshal(body, &updatedEvent); err != nil {
		return nil, fmt.Errorf("failed to parse updated event: %w", err)
	}

	return &updatedEvent, nil
}

// DeleteEvent deletes an event from the calendar
func (c *Client) DeleteEvent(ctx context.Context, accessToken, calendarID, eventID string) error {
	url := fmt.Sprintf("%s/calendars/%s/events/%s", googleCalendarAPI, url.PathEscape(calendarID), url.PathEscape(eventID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete event request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete event failed: %s", string(body))
	}

	return nil
}

// GetEvent retrieves a specific event
func (c *Client) GetEvent(ctx context.Context, accessToken, calendarID, eventID string) (*model.GoogleEvent, error) {
	url := fmt.Sprintf("%s/calendars/%s/events/%s", googleCalendarAPI, url.PathEscape(calendarID), url.PathEscape(eventID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get event request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get event failed: %s", string(body))
	}

	var event model.GoogleEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}

	return &event, nil
}

// ListEvents lists events in a calendar within a time range
func (c *Client) ListEvents(ctx context.Context, accessToken, calendarID string, timeMin, timeMax time.Time, pageToken string) (*model.EventListResponse, error) {
	params := url.Values{
		"timeMin":      {timeMin.Format(time.RFC3339)},
		"timeMax":      {timeMax.Format(time.RFC3339)},
		"singleEvents": {"true"},
		"orderBy":      {"startTime"},
		"maxResults":   {"100"},
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	reqURL := fmt.Sprintf("%s/calendars/%s/events?%s", googleCalendarAPI, url.PathEscape(calendarID), params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list events request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list events failed: %s", string(body))
	}

	var listResp model.EventListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse events list: %w", err)
	}

	return &listResp, nil
}
