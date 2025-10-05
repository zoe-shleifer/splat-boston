// Coordinate conversion utilities matching backend Go implementation
// See: internal/geo/coords.go

const EARTH_RADIUS = 6378137.0;
const ORIGIN_SHIFT = Math.PI * EARTH_RADIUS;
const TILE_METERS = 10.0;

/**
 * Convert WGS84 lat/lon to tile coordinates (x, y)
 * Matches LatLonToTileXY in internal/geo/coords.go
 */
export function latLonToTileXY(lat: number, lon: number): { x: number; y: number } {
  // Clamp latitude to Mercator
  lat = Math.max(Math.min(lat, 85.05112878), -85.05112878);
  
  const mx = lon * ORIGIN_SHIFT / 180.0;
  const my = Math.log(Math.tan((90.0 + lat) * Math.PI / 360.0)) * EARTH_RADIUS;
  
  // Shift to [0, 2*originShift], then quantize to 10m tiles
  const tx = Math.floor((mx + ORIGIN_SHIFT) / TILE_METERS);
  const ty = Math.floor((ORIGIN_SHIFT - my) / TILE_METERS); // top-down
  
  return { x: tx, y: ty };
}

/**
 * Return the chunk coordinates for a given tile coordinate
 * Matches ChunkOf in internal/geo/coords.go
 */
export function chunkOf(x: number, y: number): { cx: number; cy: number } {
  return {
    cx: x >> 8,
    cy: y >> 8
  };
}

/**
 * Return the offset within a chunk for a given tile coordinate
 * Matches OffsetOf in internal/geo/coords.go
 */
export function offsetOf(x: number, y: number): number {
  return ((y & 255) << 8) | (x & 255);
}

/**
 * Convert tile coordinates back to lat/lon (approximate center of tile)
 */
export function tileXYToLatLon(x: number, y: number): { lat: number; lon: number } {
  const mx = (x * TILE_METERS + TILE_METERS / 2) - ORIGIN_SHIFT;
  const my = ORIGIN_SHIFT - (y * TILE_METERS + TILE_METERS / 2);
  
  const lon = mx / ORIGIN_SHIFT * 180.0;
  const lat = (Math.atan(Math.exp(my / EARTH_RADIUS)) * 360.0 / Math.PI) - 90.0;
  
  return { lat, lon };
}

/**
 * Get the lat/lon bounds of a tile (for rendering)
 * Returns [[southLat, westLon], [northLat, eastLon]]
 */
export function tileBounds(x: number, y: number): [[number, number], [number, number]] {
  // Calculate Mercator coordinates for tile corners
  const mxWest = (x * TILE_METERS) - ORIGIN_SHIFT;
  const mxEast = ((x + 1) * TILE_METERS) - ORIGIN_SHIFT;
  const myNorth = ORIGIN_SHIFT - (y * TILE_METERS);
  const mySouth = ORIGIN_SHIFT - ((y + 1) * TILE_METERS);
  
  // Convert to lat/lon
  const westLon = mxWest / ORIGIN_SHIFT * 180.0;
  const eastLon = mxEast / ORIGIN_SHIFT * 180.0;
  const northLat = (Math.atan(Math.exp(myNorth / EARTH_RADIUS)) * 360.0 / Math.PI) - 90.0;
  const southLat = (Math.atan(Math.exp(mySouth / EARTH_RADIUS)) * 360.0 / Math.PI) - 90.0;
  
  return [[southLat, westLon], [northLat, eastLon]];
}

