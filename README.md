# Splat Boston - Location-Based Collaborative Painting

A collaborative painting app inspired by r/place, where users can paint 10-meter tiles on a map of Greater Boston. The key twist: you can only paint within a small radius of your current location, creating a unique location-based collaborative art experience.

## 🎨 Concept

- **Location-Restricted Painting**: Users can only paint tiles within a configurable radius (e.g., 100-500 meters) of their current GPS location
- **10-Meter Tiles**: Each paintable area is a 10m x 10m square on the map
- **Real-time Collaboration**: Multiple users can paint simultaneously, with changes visible to others in real-time
- **Boston Focus**: Initially covers Greater Boston area, with potential for expansion
- **Persistent Art**: Paintings are saved and persist over time, creating a collaborative digital mural

## 🚀 Features (Planned)

- Interactive map using React Leaflet
- GPS-based location detection
- Radius-restricted painting zones
- Real-time collaborative painting
- Color palette selection
- Tile-based painting system
- User authentication and moderation
- Mobile-responsive design

## 🛠 Tech Stack

- **Frontend**: React 18 + TypeScript
- **Mapping**: React Leaflet + Leaflet
- **Styling**: Tailwind CSS
- **State Management**: Zustand (for real-time collaboration)
- **Backend**: Node.js + Express (future)
- **Database**: PostgreSQL (future)
- **Real-time**: Socket.io (future)

## 📦 Installation

```bash
# Clone the repository
git clone <repository-url>
cd splat-boston

# Install dependencies
npm install

# Start development server
npm run dev
```

## 🗺 Map Coverage

The app initially focuses on Greater Boston, including:
- Boston proper
- Cambridge
- Somerville
- Brookline
- Newton
- Watertown
- Arlington
- Medford

## 🎯 Core Functionality

### Location-Based Restrictions
- Users must enable location services
- Painting is restricted to a configurable radius around user's location
- Radius can be adjusted (default: 200 meters)
- Visual indicators show paintable vs. non-paintable areas

### Tile System
- Map is divided into 10m x 10m tiles
- Each tile can be painted with a single color
- Tiles are identified by their geographic coordinates
- Efficient storage and retrieval of tile data

### Real-time Collaboration
- Multiple users can paint simultaneously
- Changes are broadcast to all connected users
- Conflict resolution for simultaneous edits
- User presence indicators

## 🔧 Development Setup

### Prerequisites
- Node.js 18+ 
- npm or yarn
- Modern web browser with location services

### Environment Variables
```bash
# .env.local
REACT_APP_MAP_CENTER_LAT=42.3601
REACT_APP_MAP_CENTER_LNG=-71.0589
REACT_APP_DEFAULT_ZOOM=13
REACT_APP_PAINTING_RADIUS=200
```

## 📱 Mobile Considerations

- Touch-friendly painting interface
- Optimized for mobile browsers
- Location permission handling
- Responsive map controls

## 🚧 Development Roadmap

### Phase 1: Core Map & Location
- [x] React Leaflet setup
- [ ] GPS location detection
- [ ] Radius-based painting restrictions
- [ ] Basic tile painting interface

### Phase 2: Real-time Collaboration
- [ ] WebSocket integration
- [ ] Real-time painting updates
- [ ] User presence system
- [ ] Conflict resolution

### Phase 3: Enhanced Features
- [ ] User authentication
- [ ] Color palette system
- [ ] Undo/redo functionality
- [ ] Painting history

### Phase 4: Moderation & Polish
- [ ] Content moderation tools
- [ ] Performance optimization
- [ ] Mobile app (React Native)
- [ ] Analytics and insights

## 🤝 Contributing

This is a collaborative project! Contributions are welcome:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## 📄 License

MIT License - see LICENSE file for details

## 🙏 Acknowledgments

- Inspired by r/place and wplace
- Built with React Leaflet
- Community-driven development

---

*Let's paint Boston together, one tile at a time!* 🎨
