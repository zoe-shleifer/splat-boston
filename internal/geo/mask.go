package geo

import "math"

// Mask represents a geofence mask for tile allowances
type Mask struct {
	data     []byte
	bounds   Bounds
	tileSize float64
}

// Bounds represents the bounding box for the mask
type Bounds struct {
	MinX, MinY, MaxX, MaxY int64
}

// NewMask creates a new mask with the given bounds and tile size
func NewMask(bounds Bounds, tileSize float64) *Mask {
	width := int(bounds.MaxX - bounds.MinX + 1)
	height := int(bounds.MaxY - bounds.MinY + 1)
	totalTiles := width * height
	bytesNeeded := (totalTiles + 7) / 8 // Round up to nearest byte

	return &Mask{
		data:     make([]byte, bytesNeeded),
		bounds:   bounds,
		tileSize: tileSize,
	}
}

// SetTile sets a tile as allowed (true) or forbidden (false)
func (m *Mask) SetTile(x, y int64, allowed bool) {
	if x < m.bounds.MinX || x > m.bounds.MaxX || y < m.bounds.MinY || y > m.bounds.MaxY {
		return // Out of bounds
	}

	// Convert to local coordinates
	localX := x - m.bounds.MinX
	localY := y - m.bounds.MinY
	width := int(m.bounds.MaxX - m.bounds.MinX + 1)

	// Calculate bit index
	bitIndex := int(localY*int64(width) + localX)
	byteIndex := bitIndex / 8
	bitOffset := bitIndex % 8

	if byteIndex >= len(m.data) {
		return // Out of bounds
	}

	if allowed {
		m.data[byteIndex] |= 1 << (7 - bitOffset) // MSB first
	} else {
		m.data[byteIndex] &^= 1 << (7 - bitOffset)
	}
}

// IsTileAllowed checks if a tile is allowed
func (m *Mask) IsTileAllowed(x, y int64) bool {
	if x < m.bounds.MinX || x > m.bounds.MaxX || y < m.bounds.MinY || y > m.bounds.MaxY {
		return false // Out of bounds
	}

	// Convert to local coordinates
	localX := x - m.bounds.MinX
	localY := y - m.bounds.MinY
	width := int(m.bounds.MaxX - m.bounds.MinX + 1)

	// Calculate bit index
	bitIndex := int(localY*int64(width) + localX)
	byteIndex := bitIndex / 8
	bitOffset := bitIndex % 8

	if byteIndex >= len(m.data) {
		return false // Out of bounds
	}

	return (m.data[byteIndex] & (1 << (7 - bitOffset))) != 0
}

// HaversineDistance calculates the distance between two points in meters
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
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

// TileCenter calculates the center coordinates of a tile (simplified for testing)
func TileCenter(x, y int64, tileSize float64) (lat, lon float64) {
	// This is a simplified version - in reality you'd need proper projection math
	// For testing purposes, we'll use a simple approximation
	lon = float64(x) * tileSize / 111320.0 // Rough conversion to degrees
	lat = float64(y) * tileSize / 111320.0
	return lat, lon
}
