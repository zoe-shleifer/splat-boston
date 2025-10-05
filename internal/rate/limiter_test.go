package rate

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Test cooldown and rate limiting mechanisms

func TestCooldownLimiter(t *testing.T) {
	limiter := NewLimiter()
	cooldownDuration := 5 * time.Second
	ip := "192.168.1.1"

	// Initially no cooldown
	if limiter.CheckCooldown(ip, cooldownDuration) {
		t.Errorf("Should not have cooldown initially")
	}

	// Set cooldown
	limiter.SetCooldown(ip)

	// Should have cooldown now
	if !limiter.CheckCooldown(ip, cooldownDuration) {
		t.Errorf("Should have cooldown after setting")
	}

	// Check remaining time
	remaining := limiter.GetCooldownRemaining(ip, cooldownDuration)
	if remaining <= 0 {
		t.Errorf("Should have remaining cooldown time, got %v", remaining)
	}

	// Wait for cooldown to expire
	time.Sleep(cooldownDuration + 100*time.Millisecond)

	// Should not have cooldown anymore
	if limiter.CheckCooldown(ip, cooldownDuration) {
		t.Errorf("Should not have cooldown after expiry")
	}
}

func TestCooldownMultipleIPs(t *testing.T) {
	limiter := NewLimiter()
	cooldownDuration := 5 * time.Second

	ips := []string{"192.168.1.1", "192.168.1.2", "10.0.0.1"}

	// Set cooldown for all IPs
	for _, ip := range ips {
		limiter.SetCooldown(ip)
	}

	// All should have cooldown
	for _, ip := range ips {
		if !limiter.CheckCooldown(ip, cooldownDuration) {
			t.Errorf("IP %s should have cooldown", ip)
		}
	}

	// Wait for cooldown to expire
	time.Sleep(cooldownDuration + 100*time.Millisecond)

	// None should have cooldown
	for _, ip := range ips {
		if limiter.CheckCooldown(ip, cooldownDuration) {
			t.Errorf("IP %s should not have cooldown after expiry", ip)
		}
	}
}

func TestSpeedLimiter(t *testing.T) {
	// Test with 150 km/h limit
	limiter := NewSpeedLimiter(150.0)
	ip := "192.168.1.1"

	// First position should always be allowed
	if !limiter.CheckSpeed(ip, 42.3601, -71.0589) {
		t.Errorf("First position should be allowed")
	}

	// Wait a tiny bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Short distance should be allowed (within speed limit)
	if !limiter.CheckSpeed(ip, 42.3602, -71.0589) {
		t.Errorf("Short distance should be allowed")
	}

	// Very large distance should be rejected (even with time passing, it's too far)
	if limiter.CheckSpeed(ip, 42.4000, -71.0000) {
		t.Errorf("Large distance should be rejected")
	}
}

func TestSpeedLimiterTimeBased(t *testing.T) {
	limiter := NewSpeedLimiter(100.0) // 100 km/h limit
	ip := "192.168.1.1"

	// First position
	if !limiter.CheckSpeed(ip, 42.3601, -71.0589) {
		t.Errorf("First position should be allowed")
	}

	// Same position immediately should be allowed
	if !limiter.CheckSpeed(ip, 42.3601, -71.0589) {
		t.Errorf("Same position should be allowed")
	}

	// Position 1km away after 1 minute should be allowed (60 km/h)
	// This is a simplified test - in reality you'd need to mock time
}

func TestRateLimiter(t *testing.T) {
	// Test 5 requests per minute
	limiter := NewRateLimiter(5, time.Minute)
	ip := "192.168.1.1"

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		if !limiter.Allow(ip) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	if limiter.Allow(ip) {
		t.Errorf("6th request should be denied")
	}

	// Check remaining requests
	remaining := limiter.GetRemainingRequests(ip)
	if remaining != 0 {
		t.Errorf("Expected 0 remaining requests, got %d", remaining)
	}
}

func TestRateLimiterWindow(t *testing.T) {
	// Test with short window for testing
	limiter := NewRateLimiter(3, 100*time.Millisecond)
	ip := "192.168.1.1"

	// Make 3 requests
	for i := 0; i < 3; i++ {
		if !limiter.Allow(ip) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th should be denied
	if limiter.Allow(ip) {
		t.Errorf("4th request should be denied")
	}

	// Wait for window to reset
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	if !limiter.Allow(ip) {
		t.Errorf("Request after window reset should be allowed")
	}
}

func TestRateLimiterMultipleIPs(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	ips := []string{"192.168.1.1", "192.168.1.2"}

	// Each IP should have independent limits
	for _, ip := range ips {
		// First 2 requests should be allowed
		for i := 0; i < 2; i++ {
			if !limiter.Allow(ip) {
				t.Errorf("IP %s request %d should be allowed", ip, i+1)
			}
		}

		// 3rd should be denied
		if limiter.Allow(ip) {
			t.Errorf("IP %s 3rd request should be denied", ip)
		}
	}
}

func TestCombinedLimiters(t *testing.T) {
	// Test combining cooldown and rate limiting
	cooldownLimiter := NewLimiter()
	rateLimiter := NewRateLimiter(10, time.Minute)
	speedLimiter := NewSpeedLimiter(150.0)

	ip := "192.168.1.1"
	cooldownDuration := 100 * time.Millisecond // Short cooldown for testing

	// Test that all limiters work together
	for i := 0; i < 10; i++ {
		// Wait for cooldown to expire
		if i > 0 {
			time.Sleep(cooldownDuration + 10*time.Millisecond)
		}

		// Check cooldown (should not be active after waiting)
		if cooldownLimiter.CheckCooldown(ip, cooldownDuration) {
			t.Errorf("Cooldown should not block request %d", i+1)
		}

		// Check rate limit
		if !rateLimiter.Allow(ip) {
			t.Errorf("Rate limit should allow request %d", i+1)
		}

		// Check speed (using same coordinates for simplicity)
		if !speedLimiter.CheckSpeed(ip, 42.3601, -71.0589) {
			t.Errorf("Speed limit should allow request %d", i+1)
		}

		// Set cooldown after each request
		cooldownLimiter.SetCooldown(ip)
	}

	// 11th request should hit rate limit
	if rateLimiter.Allow(ip) {
		t.Errorf("11th request should hit rate limit")
	}
}

func TestLimiterConcurrency(t *testing.T) {
	limiter := NewLimiter()
	cooldownDuration := 5 * time.Second

	// Test concurrent access
	numGoroutines := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := fmt.Sprintf("192.168.1.%d", id)

			// Set cooldown
			limiter.SetCooldown(ip)

			// Check cooldown
			if !limiter.CheckCooldown(ip, cooldownDuration) {
				t.Errorf("Goroutine %d: IP %s should have cooldown", id, ip)
			}
		}(i)
	}

	wg.Wait()
}

func TestLimiterMemoryCleanup(t *testing.T) {
	limiter := NewLimiter()
	cooldownDuration := 100 * time.Millisecond

	ip := "192.168.1.1"

	// Set cooldown
	limiter.SetCooldown(ip)

	// Wait for cooldown to expire
	time.Sleep(cooldownDuration + 50*time.Millisecond)

	// Check cooldown (should trigger cleanup)
	limiter.CheckCooldown(ip, cooldownDuration)

	// Verify cleanup happened
	limiter.mu.RLock()
	_, exists := limiter.cooldowns[ip]
	limiter.mu.RUnlock()

	if exists {
		t.Errorf("Cooldown should be cleaned up after expiry")
	}
}

func BenchmarkCooldownLimiter(b *testing.B) {
	limiter := NewLimiter()
	cooldownDuration := 5 * time.Second
	ip := "192.168.1.1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.CheckCooldown(ip, cooldownDuration)
	}
}

func BenchmarkRateLimiter(b *testing.B) {
	limiter := NewRateLimiter(100, time.Minute)
	ip := "192.168.1.1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow(ip)
	}
}

func BenchmarkSpeedLimiter(b *testing.B) {
	limiter := NewSpeedLimiter(150.0)
	ip := "192.168.1.1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use slightly different coordinates for each request
		lat := 42.3601 + float64(i)*0.0001
		lon := -71.0589 + float64(i)*0.0001
		limiter.CheckSpeed(ip, lat, lon)
	}
}
