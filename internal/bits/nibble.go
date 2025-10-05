package bits

// SetNibble sets a 4-bit color value at the given offset in a byte slice
// Returns the previous color value at that offset
func SetNibble(data []byte, offset int, color uint8) uint8 {
	if offset < 0 {
		return 0 // Return 0 for negative offsets
	}

	byteIdx := (offset * 4) / 8
	nibbleIsHigh := (offset % 2) == 0

	if byteIdx >= len(data) {
		return 0 // Return 0 for out of bounds
	}

	b := data[byteIdx]
	var prev uint8

	if nibbleIsHigh {
		// High nibble (bits 4-7)
		prev = (b & 0xF0) >> 4
		data[byteIdx] = (b & 0x0F) | (color << 4)
	} else {
		// Low nibble (bits 0-3)
		prev = b & 0x0F
		data[byteIdx] = (b & 0xF0) | color
	}

	return prev
}

// GetNibble gets a 4-bit color value at the given offset in a byte slice
func GetNibble(data []byte, offset int) uint8 {
	if offset < 0 {
		return 0 // Return 0 for negative offsets
	}

	byteIdx := (offset * 4) / 8
	nibbleIsHigh := (offset % 2) == 0

	if byteIdx >= len(data) {
		return 0 // Return 0 for out of bounds
	}

	b := data[byteIdx]

	if nibbleIsHigh {
		return (b & 0xF0) >> 4
	} else {
		return b & 0x0F
	}
}
