// Nibble (4-bit) packing/unpacking utilities for chunk data
// Each tile is represented by 4 bits (0-15), stored as two tiles per byte
// High nibble = even offset, Low nibble = odd offset

const CHUNK_SIZE = 256; // 256 x 256 tiles per chunk
const CHUNK_TILES = CHUNK_SIZE * CHUNK_SIZE; // 65,536 tiles
const CHUNK_BYTES = CHUNK_TILES / 2; // 32,768 bytes (two tiles per byte)

/**
 * Get the color at a specific offset within a chunk
 * @param data Chunk data (32KB Uint8Array)
 * @param offset Tile offset (0-65535)
 * @returns Color index (0-15)
 */
export function getNibble(data: Uint8Array, offset: number): number {
  if (offset < 0 || offset >= CHUNK_TILES) {
    throw new Error(`Invalid offset: ${offset}`);
  }
  
  const byteIdx = Math.floor(offset / 2);
  const isHighNibble = (offset % 2) === 0;
  
  const byte = data[byteIdx];
  
  if (isHighNibble) {
    return (byte & 0xF0) >> 4;
  } else {
    return byte & 0x0F;
  }
}

/**
 * Set the color at a specific offset within a chunk
 * @param data Chunk data (32KB Uint8Array) - modified in place
 * @param offset Tile offset (0-65535)
 * @param color Color index (0-15)
 */
export function setNibble(data: Uint8Array, offset: number, color: number): void {
  if (offset < 0 || offset >= CHUNK_TILES) {
    throw new Error(`Invalid offset: ${offset}`);
  }
  
  if (color < 0 || color > 15) {
    throw new Error(`Invalid color: ${color}`);
  }
  
  const byteIdx = Math.floor(offset / 2);
  const isHighNibble = (offset % 2) === 0;
  
  let byte = data[byteIdx];
  
  if (isHighNibble) {
    byte = (byte & 0x0F) | ((color << 4) & 0xF0);
  } else {
    byte = (byte & 0xF0) | (color & 0x0F);
  }
  
  data[byteIdx] = byte;
}

/**
 * Create an empty chunk (all zeros)
 */
export function createEmptyChunk(): Uint8Array {
  return new Uint8Array(CHUNK_BYTES);
}

/**
 * Clone a chunk
 */
export function cloneChunk(data: Uint8Array): Uint8Array {
  return new Uint8Array(data);
}

export { CHUNK_SIZE, CHUNK_TILES, CHUNK_BYTES };

