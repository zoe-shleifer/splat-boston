package geo

import "math"

const (
	earthRadius = 6378137.0
	originShift = math.Pi * earthRadius
	tileMeters  = 10.0
)

// LatLonToTileXY converts WGS84 lat/lon to tile coordinates (x, y)
func LatLonToTileXY(lat, lon float64) (x, y int64) {
	// Clamp latitude to Mercator
	lat = math.Max(math.Min(lat, 85.05112878), -85.05112878)
	mx := lon * originShift / 180.0
	my := math.Log(math.Tan((90.0+lat)*math.Pi/360.0)) * earthRadius
	// Shift to [0, 2*originShift], then quantize to 10m tiles
	tx := int64(math.Floor((mx + originShift) / tileMeters))
	ty := int64(math.Floor((originShift - my) / tileMeters)) // top-down
	return tx, ty
}

// ChunkOf returns the chunk coordinates for a given tile coordinate
func ChunkOf(x, y int64) (cx, cy int64) {
	return x >> 8, y >> 8
}

// OffsetOf returns the offset within a chunk for a given tile coordinate
func OffsetOf(x, y int64) int {
	return int(((y & 255) << 8) | (x & 255))
}
