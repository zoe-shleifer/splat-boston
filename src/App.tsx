import React, { useState, useEffect } from 'react';
import { MapContainer, TileLayer, Marker, Circle, useMapEvents } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import './App.css';

// Fix for default markers in React Leaflet
delete (L.Icon.Default.prototype as any)._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: require('leaflet/dist/images/marker-icon-2x.png'),
  iconUrl: require('leaflet/dist/images/marker-icon.png'),
  shadowUrl: require('leaflet/dist/images/marker-shadow.png'),
});

// Map center for Boston
const BOSTON_CENTER: [number, number] = [42.3601, -71.0589];
const DEFAULT_ZOOM = 13;
const PAINTING_RADIUS = 200; // meters

interface UserLocation {
  lat: number;
  lng: number;
  accuracy: number;
}

interface TileData {
  id: string;
  lat: number;
  lng: number;
  color: string;
  paintedBy?: string;
  paintedAt?: Date;
}

// Available colors for painting
const PAINT_COLORS = [
  '#FF0000', // Red
  '#00FF00', // Green
  '#0000FF', // Blue
  '#FFFF00', // Yellow
  '#FF00FF', // Magenta
  '#00FFFF', // Cyan
  '#000000', // Black
  '#FFFFFF', // White
  '#FFA500', // Orange
  '#800080', // Purple
];

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

const App: React.FC = () => {
  const [userLocation, setUserLocation] = useState<UserLocation | null>(null);
  const [locationError, setLocationError] = useState<string | null>(null);
  const [selectedColor, setSelectedColor] = useState<string>(PAINT_COLORS[0]);
  const [paintedTiles, setPaintedTiles] = useState<TileData[]>([]);
  const [isLoadingLocation, setIsLoadingLocation] = useState(false);

  // Get user's current location
  const getCurrentLocation = () => {
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
  };

  // Handle tile painting
  const handleTilePaint = (lat: number, lng: number) => {
    const tileId = `${Math.floor(lat * 1000)}_${Math.floor(lng * 1000)}`;
    
    // Check if tile already exists
    const existingTileIndex = paintedTiles.findIndex(tile => tile.id === tileId);
    
    if (existingTileIndex >= 0) {
      // Update existing tile
      const updatedTiles = [...paintedTiles];
      updatedTiles[existingTileIndex] = {
        ...updatedTiles[existingTileIndex],
        color: selectedColor,
        paintedAt: new Date(),
      };
      setPaintedTiles(updatedTiles);
    } else {
      // Create new tile
      const newTile: TileData = {
        id: tileId,
        lat,
        lng,
        color: selectedColor,
        paintedAt: new Date(),
      };
      setPaintedTiles([...paintedTiles, newTile]);
    }
  };

  // Load location on component mount
  useEffect(() => {
    getCurrentLocation();
  }, []);

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
              Location found! You can paint within {PAINTING_RADIUS}m radius.
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
        style={{ height: '100vh', width: '100%' }}
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        
        {/* User Location Marker */}
        {userLocation && (
          <>
            <Marker position={[userLocation.lat, userLocation.lng]} />
            <Circle
              center={[userLocation.lat, userLocation.lng]}
              radius={PAINTING_RADIUS}
              pathOptions={{
                color: '#3B82F6',
                fillColor: '#3B82F6',
                fillOpacity: 0.1,
                weight: 2,
              }}
            />
          </>
        )}

        {/* Painted Tiles */}
        {paintedTiles.map((tile) => (
          <Circle
            key={tile.id}
            center={[tile.lat, tile.lng]}
            radius={5} // 10m diameter = 5m radius
            pathOptions={{
              color: tile.color,
              fillColor: tile.color,
              fillOpacity: 0.8,
              weight: 1,
            }}
          />
        ))}

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
