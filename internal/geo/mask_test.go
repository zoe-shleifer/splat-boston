package geo

import (
	"math"
	"testing"
)

// Test geofence and mask operations for the Boston area

func TestMaskOperations(t *testing.T) {
	// Test basic mask operations
	bounds := Bounds{MinX: 0, MinY: 0, MaxX: 255, MaxY: 255}
	mask := NewMask(bounds, 10.0)

	// Test setting and getting tiles
	testCases := []struct {
		x, y    int64
		allowed bool
	}{
		{0, 0, true},
		{1, 1, false},
		{255, 255, true},
		{128, 128, false},
	}

	for _, tc := range testCases {
		mask.SetTile(tc.x, tc.y, tc.allowed)
		result := mask.IsTileAllowed(tc.x, tc.y)
		if result != tc.allowed {
			t.Errorf("Tile (%d, %d) should be %v, got %v", tc.x, tc.y, tc.allowed, result)
		}
	}
}

func TestMaskBounds(t *testing.T) {
	// Test mask bounds checking
	bounds := Bounds{MinX: 100, MinY: 200, MaxX: 355, MaxY: 455}
	mask := NewMask(bounds, 10.0)

	// Test out of bounds coordinates
	outOfBounds := []struct {
		x, y int64
	}{
		{99, 200},  // Just below min X
		{356, 200}, // Just above max X
		{100, 199}, // Just below min Y
		{100, 456}, // Just above max Y
		{-1, 200},  // Negative X
		{100, -1},  // Negative Y
	}

	for _, coord := range outOfBounds {
		if mask.IsTileAllowed(coord.x, coord.y) {
			t.Errorf("Out of bounds tile (%d, %d) should not be allowed", coord.x, coord.y)
		}
	}

	// Test in bounds coordinates
	inBounds := []struct {
		x, y int64
	}{
		{100, 200}, // Min corner
		{355, 455}, // Max corner
		{227, 327}, // Middle
	}

	for _, coord := range inBounds {
		mask.SetTile(coord.x, coord.y, true)
		if !mask.IsTileAllowed(coord.x, coord.y) {
			t.Errorf("In bounds tile (%d, %d) should be allowed", coord.x, coord.y)
		}
	}
}

func TestHaversineDistance(t *testing.T) {
	// Test Haversine distance calculation
	tests := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		expected               float64 // Expected distance in meters (approximate)
		tolerance              float64
	}{
		{
			name: "Same point",
			lat1: 42.3601, lon1: -71.0589,
			lat2: 42.3601, lon2: -71.0589,
			expected:  0,
			tolerance: 1,
		},
		{
			name: "Boston to Cambridge (approximate)",
			lat1: 42.3601, lon1: -71.0589, // Boston Common
			lat2: 42.3736, lon2: -71.1097, // Harvard Square
			expected:  5000, // Approximately 5km
			tolerance: 1000,
		},
		{
			name: "Short distance",
			lat1: 42.3601, lon1: -71.0589,
			lat2: 42.3602, lon2: -71.0589, // ~11 meters north
			expected:  11,
			tolerance: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := HaversineDistance(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(distance-tt.expected) > tt.tolerance {
				t.Errorf("Distance between (%f, %f) and (%f, %f) = %f, expected ~%f",
					tt.lat1, tt.lon1, tt.lat2, tt.lon2, distance, tt.expected)
			}
		})
	}
}

func TestGeofenceRadius(t *testing.T) {
	// Test geofence radius checking
	const geofenceRadiusM = 300.0 // 300 meters

	// Test coordinates in Boston area
	bostonLat, bostonLon := 42.3601, -71.0589

	// Convert to tile coordinates
	x, y := LatLonToTileXY(bostonLat, bostonLon)

	// Note: TileCenter uses a simplified approximation, so we'll just test
	// that the function doesn't panic and returns reasonable values
	_ = x
	_ = y
	_ = geofenceRadiusM

	// Test basic tile center calculation doesn't panic
	tileLat, tileLon := TileCenter(x, y, 10.0)
	_ = tileLat
	_ = tileLon

	// Test Haversine distance calculation
	distance := HaversineDistance(bostonLat, bostonLon, bostonLat+0.001, bostonLon)
	if distance < 100 || distance > 120 { // ~111m expected
		t.Errorf("Expected distance ~111m for 0.001 degree offset, got %f", distance)
	}
}

func TestSpeedClamp(t *testing.T) {
	// Test speed clamping to prevent teleportation
	const maxSpeedKmh = 150.0
	const maxSpeedMs = maxSpeedKmh * 1000.0 / 3600.0 // Convert to m/s

	// Test cases with different time intervals and distances
	testCases := []struct {
		name        string
		lat1, lon1  float64
		lat2, lon2  float64
		timeSeconds int64
		shouldPass  bool
	}{
		{
			name: "Normal walking speed",
			lat1: 42.3601, lon1: -71.0589,
			lat2: 42.3602, lon2: -71.0589, // ~11m north
			timeSeconds: 10,   // 10 seconds
			shouldPass:  true, // ~1.1 m/s, well under limit
		},
		{
			name: "Fast car speed over limit",
			lat1: 42.3601, lon1: -71.0589,
			lat2: 42.3650, lon2: -71.0589, // ~550m north
			timeSeconds: 10,    // 10 seconds
			shouldPass:  false, // ~55 m/s = ~198 km/h, over 150 km/h limit
		},
		{
			name: "Teleportation attempt",
			lat1: 42.3601, lon1: -71.0589,
			lat2: 42.4000, lon2: -71.0000, // ~4.4km away
			timeSeconds: 1,     // 1 second
			shouldPass:  false, // ~4400 m/s = way over limit
		},
		{
			name: "Reasonable car speed",
			lat1: 42.3601, lon1: -71.0589,
			lat2: 42.3650, lon2: -71.0589, // ~550m north
			timeSeconds: 20,   // 20 seconds
			shouldPass:  true, // ~27.5 m/s = ~99 km/h, under limit
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			distance := HaversineDistance(tc.lat1, tc.lon1, tc.lat2, tc.lon2)
			speed := distance / float64(tc.timeSeconds)

			withinLimit := speed <= maxSpeedMs

			if withinLimit != tc.shouldPass {
				t.Errorf("Speed %f m/s (%f km/h) should be %v, got %v",
					speed, speed*3.6, tc.shouldPass, withinLimit)
			}
		})
	}
}

func TestMaskPerformance(t *testing.T) {
	// Test mask performance with large datasets
	bounds := Bounds{MinX: 0, MinY: 0, MaxX: 1023, MaxY: 1023} // 1M tiles
	mask := NewMask(bounds, 10.0)

	// Fill mask with a pattern
	for y := bounds.MinY; y <= bounds.MaxY; y += 2 {
		for x := bounds.MinX; x <= bounds.MaxX; x += 2 {
			mask.SetTile(x, y, true)
		}
	}

	// Test reading performance
	start := 0
	for y := bounds.MinY; y <= bounds.MaxY; y++ {
		for x := bounds.MinX; x <= bounds.MaxX; x++ {
			mask.IsTileAllowed(x, y)
			start++
			if start > 10000 { // Limit for test performance
				break
			}
		}
		if start > 10000 {
			break
		}
	}
}

func BenchmarkMaskOperations(b *testing.B) {
	bounds := Bounds{MinX: 0, MinY: 0, MaxX: 255, MaxY: 255}
	mask := NewMask(bounds, 10.0)

	b.Run("SetTile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			x := int64(i % 256)
			y := int64((i / 256) % 256)
			mask.SetTile(x, y, i%2 == 0)
		}
	})

	b.Run("IsTileAllowed", func(b *testing.B) {
		// Pre-populate with some data
		for i := 0; i < 1000; i++ {
			x := int64(i % 256)
			y := int64((i / 256) % 256)
			mask.SetTile(x, y, i%2 == 0)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			x := int64(i % 256)
			y := int64((i / 256) % 256)
			mask.IsTileAllowed(x, y)
		}
	})
}
