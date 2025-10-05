package turnstile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test Turnstile verification for bot protection

func TestTurnstileVerification(t *testing.T) {
	// Test with mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected form content type, got %s", r.Header.Get("Content-Type"))
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
			return
		}

		// Check required fields
		secret := r.FormValue("secret")
		response := r.FormValue("response")
		remoteIP := r.FormValue("remoteip")

		if secret == "" {
			t.Errorf("Missing secret field")
		}

		if response == "" {
			t.Errorf("Missing response field")
		}

		// Mock response based on token
		var resp TurnstileResponse
		if response == "valid_token" {
			resp = TurnstileResponse{
				Success:     true,
				ChallengeTs: time.Now().Format(time.RFC3339),
				Hostname:    "example.com",
				ErrorCodes:  []string{},
			}
		} else if response == "invalid_token" {
			resp = TurnstileResponse{
				Success:    false,
				ErrorCodes: []string{"invalid-input-response"},
			}
		} else if response == "expired_token" {
			resp = TurnstileResponse{
				Success:    false,
				ErrorCodes: []string{"timeout-or-duplicate"},
			}
		} else {
			resp = TurnstileResponse{
				Success:    false,
				ErrorCodes: []string{"invalid-input-response"},
			}
		}

		// Add remote IP to response if provided
		if remoteIP != "" {
			resp.Hostname = remoteIP
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test with valid token
	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	ctx := context.Background()
	resp, err := client.Verify(ctx, "valid_token", "192.168.1.1")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success=true, got %v", resp.Success)
	}

	if len(resp.ErrorCodes) != 0 {
		t.Errorf("Expected no error codes, got %v", resp.ErrorCodes)
	}

	// Test with invalid token
	resp, err = client.Verify(ctx, "invalid_token", "192.168.1.1")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if resp.Success {
		t.Errorf("Expected success=false, got %v", resp.Success)
	}

	if len(resp.ErrorCodes) == 0 {
		t.Errorf("Expected error codes, got none")
	}
}

func TestTurnstileVerificationWithoutIP(t *testing.T) {
	// Test without remote IP
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
			return
		}

		// Check that remoteip is not set
		remoteIP := r.FormValue("remoteip")
		if remoteIP != "" {
			t.Errorf("Expected no remote IP, got %s", remoteIP)
		}

		resp := TurnstileResponse{
			Success:     true,
			ChallengeTs: time.Now().Format(time.RFC3339),
			Hostname:    "example.com",
			ErrorCodes:  []string{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	ctx := context.Background()
	resp, err := client.Verify(ctx, "valid_token", "")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success=true, got %v", resp.Success)
	}
}

func TestTurnstileVerificationTimeout(t *testing.T) {
	// Test timeout handling
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL
	client.client.Timeout = 100 * time.Millisecond // Short timeout

	ctx := context.Background()
	_, err := client.Verify(ctx, "valid_token", "192.168.1.1")
	if err == nil {
		t.Errorf("Expected timeout error, got none")
	}
}

func TestTurnstileVerificationNetworkError(t *testing.T) {
	// Test network error handling - using a definitely unreachable URL
	client := NewTurnstileClient("test_secret")
	client.baseURL = "http://192.0.2.1:9999"       // TEST-NET-1, guaranteed unreachable
	client.client.Timeout = 100 * time.Millisecond // Short timeout

	ctx := context.Background()
	_, err := client.Verify(ctx, "valid_token", "192.168.1.1")
	if err == nil {
		t.Errorf("Expected network error, got none")
	}
}

func TestTurnstileVerificationInvalidJSON(t *testing.T) {
	// Test invalid JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	ctx := context.Background()
	_, err := client.Verify(ctx, "valid_token", "192.168.1.1")
	if err == nil {
		t.Errorf("Expected JSON parse error, got none")
	}
}

func TestTurnstileVerificationHTTPError(t *testing.T) {
	// Test HTTP error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	ctx := context.Background()
	_, err := client.Verify(ctx, "valid_token", "192.168.1.1")
	if err == nil {
		t.Errorf("Expected HTTP error, got none")
	}
}

func TestTurnstileVerificationContextCancellation(t *testing.T) {
	// Test context cancellation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Verify(ctx, "valid_token", "192.168.1.1")
	if err == nil {
		t.Errorf("Expected context timeout error, got none")
	}
}

func TestTurnstileVerificationErrorCodes(t *testing.T) {
	// Test various error codes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
			return
		}

		response := r.FormValue("response")

		var resp TurnstileResponse
		switch response {
		case "expired_token":
			resp = TurnstileResponse{
				Success:    false,
				ErrorCodes: []string{"timeout-or-duplicate"},
			}
		case "invalid_token":
			resp = TurnstileResponse{
				Success:    false,
				ErrorCodes: []string{"invalid-input-response"},
			}
		case "missing_token":
			resp = TurnstileResponse{
				Success:    false,
				ErrorCodes: []string{"missing-input-response"},
			}
		case "multiple_errors":
			resp = TurnstileResponse{
				Success:    false,
				ErrorCodes: []string{"invalid-input-response", "timeout-or-duplicate"},
			}
		default:
			resp = TurnstileResponse{
				Success:     true,
				ChallengeTs: time.Now().Format(time.RFC3339),
				Hostname:    "example.com",
				ErrorCodes:  []string{},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	ctx := context.Background()

	testCases := []struct {
		token        string
		expectedErr  string
		expectedCode string
	}{
		{"expired_token", "timeout-or-duplicate", "timeout-or-duplicate"},
		{"invalid_token", "invalid-input-response", "invalid-input-response"},
		{"missing_token", "missing-input-response", "missing-input-response"},
		{"multiple_errors", "invalid-input-response", "invalid-input-response"},
		{"valid_token", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.token, func(t *testing.T) {
			resp, err := client.Verify(ctx, tc.token, "192.168.1.1")
			if err != nil {
				t.Fatalf("Verify failed: %v", err)
			}

			if tc.expectedErr == "" {
				// Should succeed
				if !resp.Success {
					t.Errorf("Expected success=true, got %v", resp.Success)
				}
			} else {
				// Should fail with specific error
				if resp.Success {
					t.Errorf("Expected success=false, got %v", resp.Success)
				}

				if len(resp.ErrorCodes) == 0 {
					t.Errorf("Expected error codes, got none")
				}

				found := false
				for _, code := range resp.ErrorCodes {
					if code == tc.expectedCode {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("Expected error code %s, got %v", tc.expectedCode, resp.ErrorCodes)
				}
			}
		})
	}
}

func TestTurnstileVerificationConcurrency(t *testing.T) {
	// Test concurrent verification requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TurnstileResponse{
			Success:     true,
			ChallengeTs: time.Now().Format(time.RFC3339),
			Hostname:    "example.com",
			ErrorCodes:  []string{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	// Test concurrent requests
	numGoroutines := 10
	results := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			ctx := context.Background()
			resp, err := client.Verify(ctx, "valid_token", "192.168.1.1")
			results <- (err == nil && resp.Success)
		}(i)
	}

	// Wait for all goroutines to complete
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-results {
			successCount++
		}
	}

	if successCount != numGoroutines {
		t.Errorf("Expected %d successful verifications, got %d", numGoroutines, successCount)
	}
}

func TestTurnstileVerificationFormData(t *testing.T) {
	// Test form data encoding
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
			return
		}

		// Verify form data
		secret := r.FormValue("secret")
		response := r.FormValue("response")
		remoteIP := r.FormValue("remoteip")

		if secret != "test_secret" {
			t.Errorf("Expected secret 'test_secret', got '%s'", secret)
		}

		if response != "test_token" {
			t.Errorf("Expected response 'test_token', got '%s'", response)
		}

		if remoteIP != "192.168.1.1" {
			t.Errorf("Expected remoteip '192.168.1.1', got '%s'", remoteIP)
		}

		resp := TurnstileResponse{
			Success:     true,
			ChallengeTs: time.Now().Format(time.RFC3339),
			Hostname:    "example.com",
			ErrorCodes:  []string{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	ctx := context.Background()
	resp, err := client.Verify(ctx, "test_token", "192.168.1.1")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success=true, got %v", resp.Success)
	}
}

func BenchmarkTurnstileVerification(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TurnstileResponse{
			Success:     true,
			ChallengeTs: time.Now().Format(time.RFC3339),
			Hostname:    "example.com",
			ErrorCodes:  []string{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewTurnstileClient("test_secret")
	client.baseURL = server.URL

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Verify(ctx, "valid_token", "192.168.1.1")
	}
}
