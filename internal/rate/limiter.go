package rate

import (
	"math"
	"sync"
	"time"
)

// Limiter handles cooldown tracking
type Limiter struct {
	cooldowns map[string]time.Time
	mu        sync.RWMutex
}

// NewLimiter creates a new rate limiter
func NewLimiter() *Limiter {
	return &Limiter{
		cooldowns: make(map[string]time.Time),
	}
}

// CheckCooldown returns true if the IP is still in cooldown
func (l *Limiter) CheckCooldown(ip string, cooldownDuration time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	lastPaint, exists := l.cooldowns[ip]
	if !exists {
		return false // No cooldown
	}

	// Check if cooldown has expired
	if time.Now().After(lastPaint.Add(cooldownDuration)) {
		delete(l.cooldowns, ip)
		return false // Cooldown expired
	}

	return true // Still in cooldown
}

// SetCooldown sets a cooldown for the given IP
func (l *Limiter) SetCooldown(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cooldowns[ip] = time.Now()
}

// GetCooldownRemaining returns the remaining cooldown duration
func (l *Limiter) GetCooldownRemaining(ip string, cooldownDuration time.Duration) time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()

	lastPaint, exists := l.cooldowns[ip]
	if !exists {
		return 0
	}

	remaining := lastPaint.Add(cooldownDuration).Sub(time.Now())
	if remaining < 0 {
		return 0
	}

	return remaining
}

// SpeedLimiter tracks position and speed
type SpeedLimiter struct {
	lastPositions map[string]Position
	mu            sync.RWMutex
	maxSpeedMs    float64
}

// Position represents a GPS position with timestamp
type Position struct {
	Lat  float64
	Lon  float64
	Time time.Time
}

// NewSpeedLimiter creates a new speed limiter
func NewSpeedLimiter(maxSpeedKmh float64) *SpeedLimiter {
	return &SpeedLimiter{
		lastPositions: make(map[string]Position),
		maxSpeedMs:    maxSpeedKmh * 1000.0 / 3600.0, // Convert km/h to m/s
	}
}

// CheckSpeed returns true if the speed is within limits
func (s *SpeedLimiter) CheckSpeed(ip string, lat, lon float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Get last position
	lastPos, exists := s.lastPositions[ip]
	if !exists {
		// First position for this IP
		s.lastPositions[ip] = Position{Lat: lat, Lon: lon, Time: now}
		return true
	}

	// Calculate distance and time
	distance := haversineDistance(lastPos.Lat, lastPos.Lon, lat, lon)
	timeDiff := now.Sub(lastPos.Time).Seconds()

	if timeDiff <= 0 {
		return true // Same time or invalid
	}

	speed := distance / timeDiff

	// Update position
	s.lastPositions[ip] = Position{Lat: lat, Lon: lon, Time: now}

	return speed <= s.maxSpeedMs
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000 // Earth radius in meters

	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Haversine formula
	dlat := lat2Rad - lat1Rad
	dlon := lon2Rad - lon1Rad

	a := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

// RateLimiter implements a sliding window rate limiter
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow returns true if the request is allowed
func (r *RateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// Get existing requests
	requests, exists := r.requests[ip]
	if !exists {
		requests = make([]time.Time, 0)
	}

	// Remove old requests
	var validRequests []time.Time
	for _, reqTime := range requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Check if under limit
	if len(validRequests) >= r.limit {
		return false
	}

	// Add current request
	validRequests = append(validRequests, now)
	r.requests[ip] = validRequests

	return true
}

// GetRemainingRequests returns the number of requests remaining in the window
func (r *RateLimiter) GetRemainingRequests(ip string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	requests, exists := r.requests[ip]
	if !exists {
		return r.limit
	}

	// Count valid requests
	validCount := 0
	for _, reqTime := range requests {
		if reqTime.After(cutoff) {
			validCount++
		}
	}

	return r.limit - validCount
}
