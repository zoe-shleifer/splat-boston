package bits

import (
	"testing"
)

// Test nibble packing and unpacking operations for 4-bit color storage

const (
	chunkSizeBytes = 32768 // 65,536 tiles * 4 bits / 8 = 32,768 bytes
	tilesPerChunk  = 65536 // 256 * 256
)

func TestNibblePacking(t *testing.T) {
	// Test basic nibble packing and unpacking
	data := make([]byte, 4) // 8 tiles worth of data

	tests := []struct {
		name     string
		offset   int
		color    uint8
		expected uint8
	}{
		{
			name:     "Set first tile (high nibble)",
			offset:   0,
			color:    5,
			expected: 0, // Should return 0 (unpainted)
		},
		{
			name:     "Set second tile (low nibble)",
			offset:   1,
			color:    3,
			expected: 0, // Should return 0 (unpainted)
		},
		{
			name:     "Set third tile (high nibble)",
			offset:   2,
			color:    7,
			expected: 0, // Should return 0 (unpainted)
		},
		{
			name:     "Set fourth tile (low nibble)",
			offset:   3,
			color:    1,
			expected: 0, // Should return 0 (unpainted)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := SetNibble(data, tt.offset, tt.color)
			if prev != tt.expected {
				t.Errorf("SetNibble returned %d, expected %d", prev, tt.expected)
			}

			// Verify the value was set correctly
			retrieved := GetNibble(data, tt.offset)
			if retrieved != tt.color {
				t.Errorf("GetNibble returned %d, expected %d", retrieved, tt.color)
			}
		})
	}
}

func TestNibbleOverwrite(t *testing.T) {
	// Test overwriting existing nibbles
	data := make([]byte, 2) // 4 tiles worth of data

	// Set initial values
	SetNibble(data, 0, 5) // High nibble of first byte
	SetNibble(data, 1, 3) // Low nibble of first byte

	// Verify initial values
	if GetNibble(data, 0) != 5 {
		t.Errorf("Initial high nibble not set correctly")
	}
	if GetNibble(data, 1) != 3 {
		t.Errorf("Initial low nibble not set correctly")
	}

	// Overwrite and check previous values
	prev0 := SetNibble(data, 0, 7)
	prev1 := SetNibble(data, 1, 2)

	if prev0 != 5 {
		t.Errorf("Previous high nibble was %d, expected 5", prev0)
	}
	if prev1 != 3 {
		t.Errorf("Previous low nibble was %d, expected 3", prev1)
	}

	// Verify new values
	if GetNibble(data, 0) != 7 {
		t.Errorf("New high nibble not set correctly")
	}
	if GetNibble(data, 1) != 2 {
		t.Errorf("New low nibble not set correctly")
	}
}

func TestNibbleBounds(t *testing.T) {
	// Test bounds checking
	data := make([]byte, 2) // 4 tiles worth of data

	// Test valid offsets
	validOffsets := []int{0, 1, 2, 3}
	for _, offset := range validOffsets {
		SetNibble(data, offset, 5)
		if GetNibble(data, offset) != 5 {
			t.Errorf("Failed to set/get nibble at valid offset %d", offset)
		}
	}

	// Test out of bounds offsets
	invalidOffsets := []int{-1, 4, 100}
	for _, offset := range invalidOffsets {
		prev := SetNibble(data, offset, 5)
		if prev != 0 {
			t.Errorf("SetNibble at invalid offset %d should return 0, got %d", offset, prev)
		}

		value := GetNibble(data, offset)
		if value != 0 {
			t.Errorf("GetNibble at invalid offset %d should return 0, got %d", offset, value)
		}
	}
}

func TestNibbleColorRange(t *testing.T) {
	// Test that all valid 4-bit color values (0-15) work correctly
	data := make([]byte, 2) // 4 tiles worth of data

	for color := uint8(0); color < 16; color++ {
		SetNibble(data, 0, color)
		retrieved := GetNibble(data, 0)
		if retrieved != color {
			t.Errorf("Color %d not stored/retrieved correctly, got %d", color, retrieved)
		}
	}
}

func TestChunkInitialization(t *testing.T) {
	// Test initializing a full chunk with all tiles unpainted (0)
	data := make([]byte, chunkSizeBytes)

	// All tiles should be 0 (unpainted) initially
	for i := 0; i < tilesPerChunk; i++ {
		if GetNibble(data, i) != 0 {
			t.Errorf("Tile %d should be unpainted (0), got %d", i, GetNibble(data, i))
		}
	}
}

func TestChunkFullCycle(t *testing.T) {
	// Test a full cycle of operations on a chunk
	data := make([]byte, chunkSizeBytes)

	// Paint some tiles
	testTiles := []struct {
		offset int
		color  uint8
	}{
		{0, 1},     // First tile
		{1, 2},     // Second tile
		{255, 3},   // Last tile of first row
		{256, 4},   // First tile of second row
		{65535, 5}, // Last tile of chunk
		{32768, 6}, // Middle of chunk
	}

	// Set all test tiles
	for _, tile := range testTiles {
		prev := SetNibble(data, tile.offset, tile.color)
		if prev != 0 {
			t.Errorf("Tile %d should start unpainted, got previous color %d", tile.offset, prev)
		}
	}

	// Verify all tiles were set correctly
	for _, tile := range testTiles {
		retrieved := GetNibble(data, tile.offset)
		if retrieved != tile.color {
			t.Errorf("Tile %d should be color %d, got %d", tile.offset, tile.color, retrieved)
		}
	}

	// Overwrite some tiles
	overwrites := []struct {
		offset       int
		color        uint8
		expectedPrev uint8
	}{
		{0, 7, 1},     // Overwrite first tile
		{65535, 8, 5}, // Overwrite last tile
	}

	for _, overwrite := range overwrites {
		prev := SetNibble(data, overwrite.offset, overwrite.color)
		if prev != overwrite.expectedPrev {
			t.Errorf("Tile %d previous color should be %d, got %d",
				overwrite.offset, overwrite.expectedPrev, prev)
		}

		retrieved := GetNibble(data, overwrite.offset)
		if retrieved != overwrite.color {
			t.Errorf("Tile %d should be color %d after overwrite, got %d",
				overwrite.offset, overwrite.color, retrieved)
		}
	}
}

func TestNibbleConcurrency(t *testing.T) {
	// Test that nibble operations are safe for concurrent access
	// (This is a basic test - real concurrency testing would need more sophisticated setup)
	data := make([]byte, 100) // 200 tiles worth of data

	// Simulate concurrent writes to different tiles
	results := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(offset int) {
			color := uint8(offset % 16)
			SetNibble(data, offset, color)
			retrieved := GetNibble(data, offset)
			results <- (retrieved == color)
		}(i * 2) // Use even offsets to avoid conflicts
	}

	// Wait for all goroutines to complete
	successCount := 0
	for i := 0; i < 10; i++ {
		if <-results {
			successCount++
		}
	}

	if successCount != 10 {
		t.Errorf("Only %d out of 10 concurrent operations succeeded", successCount)
	}
}

func BenchmarkNibbleOperations(b *testing.B) {
	data := make([]byte, chunkSizeBytes)

	b.Run("SetNibble", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			offset := i % tilesPerChunk
			color := uint8(i % 16)
			SetNibble(data, offset, color)
		}
	})

	b.Run("GetNibble", func(b *testing.B) {
		// Pre-populate with some data
		for i := 0; i < 1000; i++ {
			offset := i % tilesPerChunk
			color := uint8(i % 16)
			SetNibble(data, offset, color)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			offset := i % tilesPerChunk
			GetNibble(data, offset)
		}
	})
}
