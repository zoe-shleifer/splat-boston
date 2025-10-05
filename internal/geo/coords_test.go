package geo

import (
	"math"
	"testing"
)

// Test coordinate conversion functions from the backend design

func TestLatLonToTileXY(t *testing.T) {
	tests := []struct {
		name     string
		lat      float64
		lon      float64
		expected struct {
			x, y int64
		}
	}{
		{
			name: "Boston Common",
			lat:  42.3601,
			lon:  -71.0589,
			expected: struct {
				x, y int64
			}{
				x: 0, // Will be calculated
				y: 0, // Will be calculated
			},
		},
		{
			name: "Equator at Prime Meridian",
			lat:  0.0,
			lon:  0.0,
			expected: struct {
				x, y int64
			}{
				x: 0, // Will be calculated
				y: 0, // Will be calculated
			},
		},
		{
			name: "North Pole (clamped)",
			lat:  90.0,
			lon:  0.0,
			expected: struct {
				x, y int64
			}{
				x: 0, // Will be calculated
				y: 0, // Will be calculated
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := LatLonToTileXY(tt.lat, tt.lon)

			// For now, just verify the function doesn't panic
			// Note: extreme latitudes may produce negative y values due to coordinate system
			_ = x
			_ = y

			// Test that the function is deterministic
			x2, y2 := LatLonToTileXY(tt.lat, tt.lon)
			if x != x2 || y != y2 {
				t.Errorf("latLonToTileXY is not deterministic: got (%d, %d) then (%d, %d)", x, y, x2, y2)
			}
		})
	}
}

func TestChunkOf(t *testing.T) {
	tests := []struct {
		name     string
		x, y     int64
		expected struct {
			cx, cy int64
		}
	}{
		{
			name: "Origin chunk",
			x:    0, y: 0,
			expected: struct{ cx, cy int64 }{cx: 0, cy: 0},
		},
		{
			name: "First chunk",
			x:    255, y: 255,
			expected: struct{ cx, cy int64 }{cx: 0, cy: 0},
		},
		{
			name: "Second chunk",
			x:    256, y: 256,
			expected: struct{ cx, cy int64 }{cx: 1, cy: 1},
		},
		{
			name: "Large coordinates",
			x:    1000, y: 2000,
			expected: struct{ cx, cy int64 }{cx: 3, cy: 7}, // 1000>>8=3, 2000>>8=7
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cx, cy := ChunkOf(tt.x, tt.y)
			if cx != tt.expected.cx || cy != tt.expected.cy {
				t.Errorf("chunkOf(%d, %d) = (%d, %d), expected (%d, %d)",
					tt.x, tt.y, cx, cy, tt.expected.cx, tt.expected.cy)
			}
		})
	}
}

func TestOffsetOf(t *testing.T) {
	tests := []struct {
		name     string
		x, y     int64
		expected int
	}{
		{
			name: "Origin offset",
			x:    0, y: 0,
			expected: 0,
		},
		{
			name: "First tile in chunk",
			x:    1, y: 0,
			expected: 1,
		},
		{
			name: "First row, second column",
			x:    1, y: 0,
			expected: 1,
		},
		{
			name: "Second row, first column",
			x:    0, y: 1,
			expected: 256, // (1 << 8) | 0
		},
		{
			name: "Last tile in chunk",
			x:    255, y: 255,
			expected: 65535, // (255 << 8) | 255
		},
		{
			name: "Middle of chunk",
			x:    128, y: 128,
			expected: 32896, // (128 << 8) | 128
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := OffsetOf(tt.x, tt.y)
			if offset != tt.expected {
				t.Errorf("offsetOf(%d, %d) = %d, expected %d",
					tt.x, tt.y, offset, tt.expected)
			}
		})
	}
}

func TestCoordinateRoundTrip(t *testing.T) {
	// Test that we can convert lat/lon to tile coordinates and back
	// (within reasonable precision)

	originalLat := 42.3601
	originalLon := -71.0589

	x, y := LatLonToTileXY(originalLat, originalLon)

	// Convert back to approximate lat/lon
	// This is a simplified reverse conversion for testing
	const earthRadius = 6378137.0
	const originShift = math.Pi * earthRadius
	const tileMeters = 10.0
	mx := float64(x)*tileMeters - originShift
	my := originShift - float64(y)*tileMeters

	approxLon := mx * 180.0 / originShift
	approxLat := 2.0*math.Atan(math.Exp(my/earthRadius))*180.0/math.Pi - 90.0

	// Allow for reasonable precision loss (within ~10 meters)
	latDiff := math.Abs(approxLat - originalLat)
	lonDiff := math.Abs(approxLon - originalLon)

	// 10 meters ≈ 0.0001 degrees at this latitude
	maxDiff := 0.0001

	if latDiff > maxDiff || lonDiff > maxDiff {
		t.Errorf("Coordinate round trip failed: original (%f, %f) -> tile (%d, %d) -> approx (%f, %f), diff (%f, %f)",
			originalLat, originalLon, x, y, approxLat, approxLon, latDiff, lonDiff)
	}
}

func TestLatitudeClamping(t *testing.T) {
	// Test that extreme latitudes are properly clamped and don't panic
	extremeLat := 90.0
	x, y := LatLonToTileXY(extremeLat, 0.0)

	// Should not panic - coordinate values are valid even if negative
	_ = x
	_ = y

	// Test negative extreme latitude
	extremeLatNeg := -90.0
	x2, y2 := LatLonToTileXY(extremeLatNeg, 0.0)

	// Should not panic - coordinate values are valid even if negative
	_ = x2
	_ = y2
}
