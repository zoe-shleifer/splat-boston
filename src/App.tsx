import React, { useState, useEffect, useCallback, useRef } from 'react';
import { MapContainer, TileLayer, Marker, Rectangle, useMapEvents, useMap, GeoJSON } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import './App.css';
import { latLonToTileXY, chunkOf, offsetOf, tileXYToLatLon, tileBounds } from './utils/coords';
import { getNibble, setNibble, createEmptyChunk, CHUNK_SIZE } from './utils/nibbles';
import { fetchChunk, paintTile as apiPaintTile } from './api/client';
import { ChunkWebSocketManager, Delta } from './api/websocket';

// Fix for default markers in React Leaflet
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';

delete (L.Icon.Default.prototype as any)._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: markerIcon2x,
  iconUrl: markerIcon,
  shadowUrl: markerShadow,
});

// Map center for Boston
const BOSTON_CENTER: [number, number] = [42.3601, -71.0589];
const DEFAULT_ZOOM = 13;

// Tile system constants
const TILE_SIZE_METERS = 10; // Each tile represents 10m x 10m
const PAINTING_RADIUS = 20; // 20 meters painting radius

// Greater Boston bounds (approximate - will be replaced with official boundary)
const GREATER_BOSTON_BOUNDS = {
  north: 42.5,
  south: 42.2,
  east: -70.8,
  west: -71.3
};

// Official Greater Boston boundary data URL
const GREATER_BOSTON_BOUNDARY_URL = 'https://opendata.arcgis.com/datasets/d9f7f4433a1144f3a4bd3c39a3d7ed40_0.geojson';

// Convert meters to degrees (approximate)
const METERS_TO_DEGREES_LAT = 1 / 111000; // 1 meter in latitude degrees
const METERS_TO_DEGREES_LNG = 1 / (111000 * Math.cos(BOSTON_CENTER[0] * Math.PI / 180)); // 1 meter in longitude degrees at Boston latitude

interface UserLocation {
  lat: number;
  lng: number;
  accuracy: number;
}

interface TileData {
  x: number;
  y: number;
  lat: number;
  lng: number;
  color: string;
  colorIndex: number;
}

interface ChunkInfo {
  cx: number;
  cy: number;
  data: Uint8Array;
  seq: number;
}

// Available colors for painting (matching backend palette)
// Index 0 = unpainted (transparent), indices 1-8 = colors
const PAINT_COLORS = [
  '#FF0000', // 1: Red
  '#FFA500', // 2: Orange
  '#FFFF00', // 3: Yellow
  '#00FF00', // 4: Green
  '#00FFFF', // 5: Cyan
  '#0000FF', // 6: Blue
  '#FF00FF', // 7: Magenta
  '#FFFFFF', // 8: White
];

// Map color index (1-8) to hex color
function colorIndexToHex(index: number): string {
  if (index === 0) return 'transparent';
  if (index < 1 || index > 8) return 'transparent';
  return PAINT_COLORS[index - 1];
}

// Map hex color to color index (1-8)
function hexToColorIndex(hex: string): number {
  const index = PAINT_COLORS.indexOf(hex);
  return index >= 0 ? index + 1 : 1; // Default to red if not found
}

// Helper to create chunk key
function chunkKey(cx: number, cy: number): string {
  return `${cx}:${cy}`;
}

// Calculate which chunks are visible in the given bounds
function getVisibleChunks(
  bounds: L.LatLngBounds, 
  zoom: number,
  userLocation?: { lat: number; lng: number } | null
): Array<{ cx: number; cy: number }> {
  const chunks: Array<{ cx: number; cy: number }> = [];
  const seen = new Set<string>();
  
  // Don't load chunks when zoomed out too far (performance protection)
  // At zoom 10 or less, the entire Greater Boston is visible (800+ chunks)
  if (zoom < 11) {
    // Only load chunks in a small area around user
    if (userLocation) {
      const { x, y } = latLonToTileXY(userLocation.lat, userLocation.lng);
      const userChunk = chunkOf(x, y);
      
      // Load 5x5 grid around user = 25 chunks
      for (let dx = -2; dx <= 2; dx++) {
        for (let dy = -2; dy <= 2; dy++) {
          chunks.push({ cx: userChunk.cx + dx, cy: userChunk.cy + dy });
        }
      }
      return chunks;
    }
    return [];
  }
  
  // Get corners of the visible area
  const sw = bounds.getSouthWest();
  const ne = bounds.getNorthEast();
  
  // Convert corners to tile coordinates
  const swTile = latLonToTileXY(sw.lat, sw.lng);
  const neTile = latLonToTileXY(ne.lat, ne.lng);
  
  // Get chunk coordinates for corners
  const swChunk = chunkOf(swTile.x, swTile.y);
  const neChunk = chunkOf(neTile.x, neTile.y);
  
  // Calculate how many chunks would be visible
  const chunksWide = Math.abs(neChunk.cx - swChunk.cx) + 1;
  const chunksHigh = Math.abs(neChunk.cy - swChunk.cy) + 1;
  const totalChunks = chunksWide * chunksHigh;
  
  // Hard limit: don't load more than 400 chunks at once (12.8 MB)
  // This allows zooming out significantly while staying under 20MB total
  const MAX_CHUNKS = 400;
  
  if (totalChunks > MAX_CHUNKS) {
    // Prioritize chunks near user location if available
    if (userLocation) {
      const { x, y } = latLonToTileXY(userLocation.lat, userLocation.lng);
      const userChunk = chunkOf(x, y);
      
      // Load a large area around user (15x15 grid = 225 chunks ≈ 7.2MB)
      const radius = 7;
      for (let dx = -radius; dx <= radius; dx++) {
        for (let dy = -radius; dy <= radius; dy++) {
          chunks.push({ cx: userChunk.cx + dx, cy: userChunk.cy + dy });
        }
      }
      return chunks;
    }
    
    // Otherwise load large center area (15x15 = 225 chunks)
    const centerCx = Math.floor((swChunk.cx + neChunk.cx) / 2);
    const centerCy = Math.floor((swChunk.cy + neChunk.cy) / 2);
    const radius = 7;
    
    for (let dx = -radius; dx <= radius; dx++) {
      for (let dy = -radius; dy <= radius; dy++) {
        chunks.push({ cx: centerCx + dx, cy: centerCy + dy });
      }
    }
    return chunks;
  }
  
  // Normal case: iterate through all visible chunks
  for (let cx = Math.min(swChunk.cx, neChunk.cx); cx <= Math.max(swChunk.cx, neChunk.cx); cx++) {
    for (let cy = Math.min(swChunk.cy, neChunk.cy); cy <= Math.max(swChunk.cy, neChunk.cy); cy++) {
      const key = chunkKey(cx, cy);
      if (!seen.has(key)) {
        seen.add(key);
        chunks.push({ cx, cy });
      }
    }
  }
  
  return chunks;
}

const MapEvents = ({ onMapClick, userLocation, paintingRadius }: {
  onMapClick: (lat: number, lng: number) => void;
  userLocation: UserLocation | null;
  paintingRadius: number;
}) => {
  useMapEvents({
    click: (e) => {
      const { lat, lng } = e.latlng;
      
      // Check if click is within painting radius
      if (userLocation) {
        const distance = L.latLng(userLocation.lat, userLocation.lng)
          .distanceTo(L.latLng(lat, lng));
        
        if (distance <= paintingRadius) {
          onMapClick(lat, lng);
        }
      }
    },
  });
  
  return null;
};

// Component to track viewport changes and load chunks
const ViewportTracker: React.FC<{ 
  onViewportChange: (chunks: Array<{ cx: number; cy: number }>) => void;
  userLocation: UserLocation | null;
}> = ({ onViewportChange, userLocation }) => {
  const map = useMap();
  const lastUpdateRef = useRef<string>('');
  
  const updateVisibleChunks = useCallback(() => {
    const bounds = map.getBounds();
    const zoom = map.getZoom();
    const chunks = getVisibleChunks(bounds, zoom, userLocation);
    
    // Create a key to detect if chunks actually changed
    const chunksKey = chunks.map(c => chunkKey(c.cx, c.cy)).sort().join(',');
    
    if (chunksKey !== lastUpdateRef.current) {
      lastUpdateRef.current = chunksKey;
      onViewportChange(chunks);
    }
  }, [map, onViewportChange, userLocation]);
  
  useMapEvents({
    moveend: updateVisibleChunks,
    zoomend: updateVisibleChunks,
  });
  
  // Initial load
  useEffect(() => {
    updateVisibleChunks();
  }, [updateVisibleChunks]);
  
  return null;
};

// Component to render a single-colored tile
const TileRenderer: React.FC<{ tile: TileData }> = ({ tile }) => {
  const bounds = React.useMemo(() => {
    return tileBounds(tile.x, tile.y);
  }, [tile.x, tile.y]);

  return (
    <Rectangle
      bounds={bounds}
      pathOptions={{
        color: tile.color,
        fillColor: tile.color,
        fillOpacity: 0.8,
        weight: 0.5,
      }}
      eventHandlers={{
        click: () => {
          // Optional: handle tile click events
        }
      }}
    />
  );
};

// Component to show the grid of editable tiles within the painting radius
const TileGridOverlay: React.FC<{ userLocation: UserLocation; paintingRadius: number }> = ({ 
  userLocation, 
  paintingRadius 
}) => {
  const tiles = React.useMemo(() => {
    const tileList: Array<{ x: number; y: number; id: string }> = [];
    
    // Get the center tile in tile coordinate space
    const centerTile = latLonToTileXY(userLocation.lat, userLocation.lng);
    
    // Calculate how many tiles fit in the radius
    const tilesPerSide = Math.ceil((paintingRadius * 2) / TILE_SIZE_METERS);
    
    // Generate tiles in a grid pattern around the center tile
    for (let dx = -Math.floor(tilesPerSide / 2); dx <= Math.floor(tilesPerSide / 2); dx++) {
      for (let dy = -Math.floor(tilesPerSide / 2); dy <= Math.floor(tilesPerSide / 2); dy++) {
        const tileX = centerTile.x + dx;
        const tileY = centerTile.y + dy;
        
        // Get tile center in lat/lng
        const tileCenter = tileXYToLatLon(tileX, tileY);
        
        // Check if tile center is within painting radius
        const distance = L.latLng(userLocation.lat, userLocation.lng)
          .distanceTo(L.latLng(tileCenter.lat, tileCenter.lon));
        
        if (distance <= paintingRadius) {
          tileList.push({
            x: tileX,
            y: tileY,
            id: `${tileX}_${tileY}`
          });
        }
      }
    }
    
    return tileList;
  }, [userLocation, paintingRadius]);

  return (
    <>
      {tiles.map((tile) => (
        <TileBoundary key={tile.id} tile={tile} />
      ))}
    </>
  );
};

// Component to render a single tile boundary
const TileBoundary: React.FC<{ tile: { x: number; y: number; id: string } }> = ({ tile }) => {
  const bounds = React.useMemo(() => {
    return tileBounds(tile.x, tile.y);
  }, [tile.x, tile.y]);

  return (
    <Rectangle
      bounds={bounds}
      pathOptions={{
        color: '#3B82F6',
        fillColor: '#3B82F6',
        fillOpacity: 0.1,
        weight: 2,
        dashArray: '5, 5'
      }}
    />
  );
};


// Component to load and display the official Greater Boston boundary
const GreaterBostonBoundary: React.FC = () => {
  const [boundaryData, setBoundaryData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchBoundaryData = async () => {
      try {
        setLoading(true);
        const response = await fetch(GREATER_BOSTON_BOUNDARY_URL);
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        setBoundaryData(data);
        setError(null);
      } catch (err) {
        console.error('Error fetching Greater Boston boundary:', err);
        setError('Failed to load Greater Boston boundary data');
      } finally {
        setLoading(false);
      }
    };

    fetchBoundaryData();
  }, []);

  if (loading) {
    return null; // Don't render anything while loading
  }

  if (error || !boundaryData) {
    console.warn('Using fallback boundary data:', error);
    return null; // Fall back to the approximate bounds
  }

  return (
    <GeoJSON
      data={boundaryData}
      style={{
        color: '#3B82F6',
        weight: 2,
        fillColor: 'transparent',
        fillOpacity: 0,
        dashArray: '10, 5',
        opacity: 0.7
      }}
    />
  );
};

// Helper function to check if a point is within the Greater Boston boundary
const isPointInGreaterBoston = (lat: number, lng: number, boundaryData: any): boolean => {
  if (!boundaryData || !boundaryData.features) {
    // Fallback to approximate bounds if no boundary data
    return lat >= GREATER_BOSTON_BOUNDS.south && 
           lat <= GREATER_BOSTON_BOUNDS.north &&
           lng >= GREATER_BOSTON_BOUNDS.west && 
           lng <= GREATER_BOSTON_BOUNDS.east;
  }

  // Simple point-in-polygon check for the first feature
  const feature = boundaryData.features[0];
  if (!feature || !feature.geometry || !feature.geometry.coordinates) {
    return false;
  }

  // This is a simplified check - for production, you'd want to use a proper point-in-polygon library
  const coordinates = feature.geometry.coordinates[0]; // Assuming first ring is outer boundary
  let inside = false;
  
  for (let i = 0, j = coordinates.length - 1; i < coordinates.length; j = i++) {
    const xi = coordinates[i][0], yi = coordinates[i][1];
    const xj = coordinates[j][0], yj = coordinates[j][1];
    
    if (((yi > lng) !== (yj > lng)) && (lng < (xj - xi) * (lng - yi) / (yj - yi) + xi)) {
      inside = !inside;
    }
  }
  
  return inside;
};

const App: React.FC = () => {
  const [userLocation, setUserLocation] = useState<UserLocation | null>(null);
  const [locationError, setLocationError] = useState<string | null>(null);
  const [selectedColor, setSelectedColor] = useState<string>(PAINT_COLORS[0]);
  const [paintedTiles, setPaintedTiles] = useState<Map<string, TileData>>(new Map());
  const [isLoadingLocation, setIsLoadingLocation] = useState(false);
  const [selectedTile, setSelectedTile] = useState<string | null>(null);
  const [loadedChunks, setLoadedChunks] = useState<Map<string, ChunkInfo>>(new Map());
  const [loadingChunk, setLoadingChunk] = useState(false);
  const [paintError, setPaintError] = useState<string | null>(null);
  const [wsConnected, setWsConnected] = useState(false);
  const [visibleChunks, setVisibleChunks] = useState<Set<string>>(new Set());
  const wsManagerRef = useRef<ChunkWebSocketManager>(new ChunkWebSocketManager());

  // Get user's current location
  const getCurrentLocation = useCallback(() => {
    setIsLoadingLocation(true);
    setLocationError(null);

    if (!navigator.geolocation) {
      setLocationError('Geolocation is not supported by this browser.');
      setIsLoadingLocation(false);
      return;
    }

    navigator.geolocation.getCurrentPosition(
      (position) => {
        const { latitude, longitude, accuracy } = position.coords;
        setUserLocation({
          lat: latitude,
          lng: longitude,
          accuracy: accuracy,
        });
        setIsLoadingLocation(false);
      },
      (error) => {
        let errorMessage = 'Unable to retrieve your location.';
        switch (error.code) {
          case error.PERMISSION_DENIED:
            errorMessage = 'Location access denied by user.';
            break;
          case error.POSITION_UNAVAILABLE:
            errorMessage = 'Location information is unavailable.';
            break;
          case error.TIMEOUT:
            errorMessage = 'Location request timed out.';
            break;
        }
        setLocationError(errorMessage);
        setIsLoadingLocation(false);
      },
      {
        enableHighAccuracy: true,
        timeout: 10000,
        maximumAge: 300000, // 5 minutes
      }
    );
  }, []);

  // Load chunk data from backend
  const loadChunk = useCallback(async (cx: number, cy: number) => {
    const key = chunkKey(cx, cy);
    
    // Skip if already loaded
    if (loadedChunks.has(key)) {
      return;
    }
    
    setLoadingChunk(true);
    try {
      const { data, seq } = await fetchChunk(cx, cy);
      
      // Store chunk data
      setLoadedChunks(prev => {
        const updated = new Map(prev);
        updated.set(key, { cx, cy, data, seq });
        return updated;
      });
      
      // Parse chunk data and merge painted tiles
      setPaintedTiles(prev => {
        const updated = new Map(prev);
        
        for (let o = 0; o < CHUNK_SIZE * CHUNK_SIZE; o++) {
          const colorIndex = getNibble(data, o);
          
          // Calculate tile coordinates
          const localY = Math.floor(o / CHUNK_SIZE);
          const localX = o % CHUNK_SIZE;
          const x = cx * CHUNK_SIZE + localX;
          const y = cy * CHUNK_SIZE + localY;
          const tileKey = `${x}_${y}`;
          
          if (colorIndex > 0) {
            const { lat, lon: lng } = tileXYToLatLon(x, y);
            updated.set(tileKey, {
              x,
              y,
              lat,
              lng,
              color: colorIndexToHex(colorIndex),
              colorIndex,
            });
          } else {
            // Remove tile if it's now unpainted
            updated.delete(tileKey);
          }
        }
        
        return updated;
      });
    } catch (error) {
      console.error(`Failed to load chunk (${cx}, ${cy}):`, error);
      setLocationError(`Failed to load chunk (${cx}, ${cy})`);
    } finally {
      setLoadingChunk(false);
    }
  }, [loadedChunks]);

  // Handle delta updates from WebSocket
  const handleDelta = useCallback((cx: number, cy: number, delta: Delta) => {
    const key = chunkKey(cx, cy);
    
    // Update chunk data
    setLoadedChunks(prev => {
      const chunk = prev.get(key);
      if (!chunk) return prev;
      
      // Check if delta is newer
      if (delta.seq <= chunk.seq) {
        return prev; // Ignore old deltas
      }
      
      const newData = new Uint8Array(chunk.data);
      setNibble(newData, delta.o, delta.color);
      
      const updated = new Map(prev);
      updated.set(key, { ...chunk, data: newData, seq: delta.seq });
      return updated;
    });
    
    // Update painted tiles
    const localY = Math.floor(delta.o / CHUNK_SIZE);
    const localX = delta.o % CHUNK_SIZE;
    const x = cx * CHUNK_SIZE + localX;
    const y = cy * CHUNK_SIZE + localY;
    const tileKey = `${x}_${y}`;
    
    setPaintedTiles(prev => {
      const updated = new Map(prev);
      
      if (delta.color === 0) {
        updated.delete(tileKey);
      } else {
        const { lat, lon: lng } = tileXYToLatLon(x, y);
        updated.set(tileKey, {
          x,
          y,
          lat,
          lng,
          color: colorIndexToHex(delta.color),
          colorIndex: delta.color,
        });
      }
      
      return updated;
    });
  }, []);

  // Handle viewport changes and manage chunk loading/subscriptions
  const handleViewportChange = useCallback((chunks: Array<{ cx: number; cy: number }>) => {
    const newVisibleChunks = new Set(chunks.map(c => chunkKey(c.cx, c.cy)));
    setVisibleChunks(newVisibleChunks);
    
    // Load all visible chunks
    chunks.forEach(({ cx, cy }) => {
      loadChunk(cx, cy);
    });
  }, [loadChunk]);

  // Subscribe to WebSocket updates for all visible chunks
  useEffect(() => {
    const manager = wsManagerRef.current;
    
    // Subscribe to all visible chunks
    visibleChunks.forEach(key => {
      const [cxStr, cyStr] = key.split(':');
      const cx = parseInt(cxStr, 10);
      const cy = parseInt(cyStr, 10);
      
      // Check if already subscribed
      if (!manager.getConnection(cx, cy)) {
        manager.subscribe(
          cx,
          cy,
          (delta) => handleDelta(cx, cy, delta),
          (error) => {
            console.error(`WebSocket error for chunk (${cx}, ${cy}):`, error);
          },
          () => {
            console.log(`WebSocket closed for chunk (${cx}, ${cy})`);
          },
          () => {
            console.log(`WebSocket opened for chunk (${cx}, ${cy})`);
            setWsConnected(true);
          }
        );
      }
    });
    
    // Unsubscribe from chunks that are no longer visible
    const currentConnections = new Map<string, { cx: number; cy: number }>();
    loadedChunks.forEach((chunk, key) => {
      currentConnections.set(key, { cx: chunk.cx, cy: chunk.cy });
    });
    
    currentConnections.forEach(({ cx, cy }, key) => {
      if (!visibleChunks.has(key)) {
        manager.unsubscribe(cx, cy);
      }
    });
    
    return () => {
      // Cleanup: unsubscribe from all on unmount
      manager.unsubscribeAll();
    };
  }, [visibleChunks, handleDelta, loadedChunks]);

  // Unload chunks that are far outside the viewport to save memory
  useEffect(() => {
    const MAX_LOADED_CHUNKS = 625; // Keep at most 625 chunks in memory (20MB)
    
    if (loadedChunks.size > MAX_LOADED_CHUNKS) {
      // Remove chunks that are not visible
      setLoadedChunks(prev => {
        const updated = new Map(prev);
        const chunksToRemove: string[] = [];
        
        prev.forEach((chunk, key) => {
          if (!visibleChunks.has(key)) {
            chunksToRemove.push(key);
          }
        });
        
        // Remove oldest non-visible chunks first
        chunksToRemove.slice(0, prev.size - MAX_LOADED_CHUNKS).forEach(key => {
          updated.delete(key);
        });
        
        return updated;
      });
      
      // Also remove tiles from unloaded chunks
      setPaintedTiles(prev => {
        const updated = new Map(prev);
        
        prev.forEach((tile, tileKey) => {
          const { cx, cy } = chunkOf(tile.x, tile.y);
          const chunkKeyStr = chunkKey(cx, cy);
          
          if (!loadedChunks.has(chunkKeyStr)) {
            updated.delete(tileKey);
          }
        });
        
        return updated;
      });
    }
  }, [loadedChunks, visibleChunks]);

  // Handle tile painting
  const handleTilePaint = useCallback(async (lat: number, lng: number) => {
    if (!userLocation) {
      setPaintError('Location not available');
      return;
    }
    
    // Calculate tile coordinates
    const { x, y } = latLonToTileXY(lat, lng);
    const { cx, cy } = chunkOf(x, y);
    const o = offsetOf(x, y);
    
    // Get color index
    const colorIndex = hexToColorIndex(selectedColor);
    
    setPaintError(null);
    
    try {
      // Send paint request to backend
      const response = await apiPaintTile({
        lat: userLocation.lat,
        lon: userLocation.lng,
        cx,
        cy,
        o,
        color: colorIndex,
        turnstileToken: '', // TODO: Add Turnstile integration
      });
      
      console.log('Paint successful:', response);
      setSelectedTile(`${x}_${y}`);
      
      // The WebSocket will send the delta update
    } catch (error: any) {
      console.error('Paint failed:', error);
      setPaintError(error.message || 'Failed to paint tile');
      
      // Clear error after 5 seconds
      setTimeout(() => setPaintError(null), 5000);
    }
  }, [userLocation, selectedColor]);

  // Load location on component mount
  useEffect(() => {
    getCurrentLocation();
  }, [getCurrentLocation]);

  return (
    <div className="App">
      <div className="painting-interface">
        <h2 className="text-lg font-bold mb-4">Splat Boston</h2>
        
        {/* Location Status */}
        <div className="location-status">
          {isLoadingLocation && (
            <div className="loading">Getting your location...</div>
          )}
          {locationError && (
            <div className="error">{locationError}</div>
          )}
          {userLocation && (
            <div className="success">
              Location found! You can paint tiles within 20m radius.
            </div>
          )}
        </div>

        {/* Backend Status */}
        <div className="mb-4 text-sm">
          <div className="flex justify-between items-center">
            <span>Backend Status:</span>
            <span className={wsConnected ? 'text-green-600' : 'text-red-600'}>
              {wsConnected ? '● Connected' : '○ Disconnected'}
            </span>
          </div>
          <div className="text-xs text-gray-600 mt-1">
            Loaded Chunks: {loadedChunks.size} | Visible: {visibleChunks.size}
          </div>
          {loadingChunk && (
            <div className="text-xs text-blue-600 mt-1">Loading chunk...</div>
          )}
        </div>

        {/* Paint Error */}
        {paintError && (
          <div className="mb-4 p-2 bg-red-100 text-red-700 text-sm rounded">
            {paintError}
          </div>
        )}

        {/* Stats */}
        <div className="mb-4 text-sm">
          <div className="flex justify-between">
            <span>Painted Tiles: {paintedTiles.size}</span>
          </div>
          {selectedTile && (
            <div className="text-xs text-gray-600 mt-1">
              Last painted: {selectedTile}
            </div>
          )}
        </div>

        {/* Color Palette */}
        <div className="mb-4">
          <label className="block text-sm font-medium mb-2">Choose Color:</label>
          <div className="color-palette">
            {PAINT_COLORS.map((color) => (
              <div
                key={color}
                className={`color-option ${selectedColor === color ? 'selected' : ''}`}
                style={{ backgroundColor: color }}
                onClick={() => setSelectedColor(color)}
              />
            ))}
          </div>
        </div>

        {/* Location Button */}
        <button
          onClick={getCurrentLocation}
          className="w-full bg-blue-500 text-white px-4 py-2 rounded hover:bg-blue-600 transition-colors"
        >
          Update Location
        </button>
      </div>

      {/* Map */}
      <MapContainer
        center={BOSTON_CENTER}
        zoom={DEFAULT_ZOOM}
        style={{ height: '100vh', width: '100%', backgroundColor: '#f0f0f0' }}
        minZoom={9}
        maxZoom={18}
        zoomControl={true}
        scrollWheelZoom={true}
        doubleClickZoom={true}
        dragging={true}
      >
        {/* Base map - OpenStreetMap tiles */}
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          maxZoom={18}
          minZoom={9}
          opacity={1}
          zIndex={1}
        />
        
        {/* Official Greater Boston boundary */}
        <GreaterBostonBoundary />
        
        {/* User Location Marker */}
        {userLocation && (
          <>
            <Marker position={[userLocation.lat, userLocation.lng]} />
            <TileGridOverlay 
              userLocation={userLocation} 
              paintingRadius={PAINTING_RADIUS}
            />
          </>
        )}

        {/* Painted Tiles */}
        {Array.from(paintedTiles.values()).map((tile) => (
          <TileRenderer key={`${tile.x}_${tile.y}`} tile={tile} />
        ))}

        {/* Street overlays removed - focusing on core painting functionality */}

        {/* Viewport Tracker for multi-chunk loading */}
        <ViewportTracker onViewportChange={handleViewportChange} userLocation={userLocation} />

        {/* Map Events */}
        <MapEvents
          onMapClick={handleTilePaint}
          userLocation={userLocation}
          paintingRadius={PAINTING_RADIUS}
        />
      </MapContainer>
    </div>
  );
};

export default App;
