package turnstile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// TurnstileResponse represents the response from Cloudflare Turnstile API
type TurnstileResponse struct {
	Success     bool     `json:"success"`
	ChallengeTs string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

// TurnstileClient handles Turnstile verification
type TurnstileClient struct {
	secretKey string
	client    *http.Client
	baseURL   string
}

// NewTurnstileClient creates a new Turnstile client
func NewTurnstileClient(secretKey string) *TurnstileClient {
	return &TurnstileClient{
		secretKey: secretKey,
		client:    &http.Client{Timeout: 10 * time.Second},
		baseURL:   "https://challenges.cloudflare.com/turnstile/v0/siteverify",
	}
}

// Verify verifies a Turnstile token
func (tc *TurnstileClient) Verify(ctx context.Context, token, remoteIP string) (*TurnstileResponse, error) {
	// Prepare form data
	formData := map[string]string{
		"secret":   tc.secretKey,
		"response": token,
	}

	if remoteIP != "" {
		formData["remoteip"] = remoteIP
	}

	// Create form
	form := make([]byte, 0)
	for key, value := range formData {
		if len(form) > 0 {
			form = append(form, '&')
		}
		form = append(form, []byte(key)...)
		form = append(form, '=')
		form = append(form, []byte(value)...)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", tc.baseURL, bytes.NewReader(form))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make request
	resp, err := tc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var turnstileResp TurnstileResponse
	if err := json.Unmarshal(body, &turnstileResp); err != nil {
		return nil, err
	}

	return &turnstileResp, nil
}
