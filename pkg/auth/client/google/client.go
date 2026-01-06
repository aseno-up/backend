package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aseno/backend/internal/errors"
)

// Config holds the Google OAuth configuration
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Client provides Google OAuth authentication functionality
type Client struct {
	config Config
	http   *http.Client
}

// GoogleUserInfo represents the user information from Google
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token,omitempty"`
}

// NewClient creates a new Google OAuth client
func NewClient(config Config) *Client {
	return &Client{
		config: config,
		http:   &http.Client{},
	}
}

// GetAuthURL returns the Google OAuth authorization URL
func (c *Client) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", c.config.ClientID)
	params.Set("redirect_uri", c.config.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("state", state)

	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens
func (c *Client) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)
	data.Set("redirect_uri", c.config.RedirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.ErrInternalInstance.WithMessage(fmt.Sprintf("Google OAuth error: %s", string(body)))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &tokenResp, nil
}

// GetUserInfo retrieves user information using the access token
func (c *Client) GetUserInfo(ctx context.Context, accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.ErrInternalInstance.WithMessage(fmt.Sprintf("Google API error: %s", string(body)))
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	return &userInfo, nil
}

// VerifyIDToken verifies a Google ID token and extracts user information
// This is useful when receiving the ID token directly from the frontend
func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	// Use Google's tokeninfo endpoint to verify the ID token
	req, err := http.NewRequestWithContext(ctx, "GET", "https://oauth2.googleapis.com/tokeninfo?id_token="+url.QueryEscape(idToken), nil)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.ErrUnauthorizedInstance.WithMessage("Invalid Google ID token")
	}

	// Parse the token info
	var tokenInfo struct {
		Iss           string `json:"iss"`
		Azp           string `json:"azp"`
		Aud           string `json:"aud"`
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Locale        string `json:"locale"`
		Exp           string `json:"exp"`
	}

	if err := json.Unmarshal(body, &tokenInfo); err != nil {
		return nil, errors.ErrInternalInstance.WithError(err)
	}

	// Verify the audience matches our client ID
	if tokenInfo.Aud != c.config.ClientID {
		return nil, errors.ErrUnauthorizedInstance.WithMessage("Invalid token audience")
	}

	return &GoogleUserInfo{
		ID:            tokenInfo.Sub,
		Email:         tokenInfo.Email,
		VerifiedEmail: tokenInfo.EmailVerified == "true",
		Name:          tokenInfo.Name,
		GivenName:     tokenInfo.GivenName,
		FamilyName:    tokenInfo.FamilyName,
		Picture:       tokenInfo.Picture,
		Locale:        tokenInfo.Locale,
	}, nil
}
