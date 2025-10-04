#!/bin/bash

echo "🎨 Setting up Splat Boston - Location-based Collaborative Painting App"
echo "=================================================================="

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+ first:"
    echo "   Visit: https://nodejs.org/"
    echo "   Or use a package manager like Homebrew: brew install node"
    exit 1
fi

# Check Node.js version
NODE_VERSION=$(node -v | cut -d'v' -f2 | cut -d'.' -f1)
if [ "$NODE_VERSION" -lt 18 ]; then
    echo "❌ Node.js version 18+ is required. Current version: $(node -v)"
    echo "   Please upgrade Node.js: https://nodejs.org/"
    exit 1
fi

echo "✅ Node.js $(node -v) detected"

# Install dependencies
echo "📦 Installing dependencies..."
npm install

if [ $? -ne 0 ]; then
    echo "❌ Failed to install dependencies"
    exit 1
fi

echo "✅ Dependencies installed successfully"

# Create environment file
echo "⚙️  Setting up environment..."
cat > .env.local << EOF
# Map Configuration
REACT_APP_MAP_CENTER_LAT=42.3601
REACT_APP_MAP_CENTER_LNG=-71.0589
REACT_APP_DEFAULT_ZOOM=13
REACT_APP_PAINTING_RADIUS=200

# Development
GENERATE_SOURCEMAP=false
EOF

echo "✅ Environment file created"

echo ""
echo "🚀 Setup complete! To start the development server:"
echo "   npm start"
echo ""
echo "📱 The app will open at http://localhost:3000"
echo "🗺️  Make sure to allow location access when prompted"
echo ""
echo "🎨 Happy painting!"
