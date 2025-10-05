// API client for backend communication

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

export interface PaintRequest {
  lat: number;
  lon: number;
  cx: number;
  cy: number;
  o: number;
  color: number;
  turnstileToken: string;
}

export interface PaintResponse {
  ok: boolean;
  seq: number;
  ts: number;
}

export interface ChunkData {
  data: Uint8Array;
  seq: number;
}

/**
 * Fetch chunk data from the backend
 * @param cx Chunk X coordinate
 * @param cy Chunk Y coordinate
 * @returns Chunk data and sequence number
 */
export async function fetchChunk(cx: number, cy: number): Promise<ChunkData> {
  const url = `${API_BASE_URL}/state/chunk?cx=${cx}&cy=${cy}`;
  
  const response = await fetch(url);
  
  if (!response.ok) {
    throw new Error(`Failed to fetch chunk: ${response.statusText}`);
  }
  
  const seqHeader = response.headers.get('X-Seq');
  const seq = seqHeader ? parseInt(seqHeader, 10) : 0;
  
  const arrayBuffer = await response.arrayBuffer();
  const data = new Uint8Array(arrayBuffer);
  
  return { data, seq };
}

/**
 * Paint a tile on the backend
 * @param request Paint request
 * @returns Paint response
 */
export async function paintTile(request: PaintRequest): Promise<PaintResponse> {
  const url = `${API_BASE_URL}/paint`;
  
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  });
  
  if (!response.ok) {
    const errorText = await response.text();
    
    // Handle specific error codes
    if (response.status === 429) {
      throw new Error('Cooldown: Please wait before painting again');
    } else if (response.status === 403) {
      throw new Error('Geofence: Outside allowed area or speed limit exceeded');
    } else if (response.status === 401) {
      throw new Error('Turnstile verification failed');
    } else if (response.status === 400) {
      throw new Error(`Invalid request: ${errorText}`);
    } else {
      throw new Error(`Failed to paint tile: ${errorText}`);
    }
  }
  
  return await response.json();
}

/**
 * Check if the backend is healthy
 */
export async function healthCheck(): Promise<boolean> {
  try {
    const url = `${API_BASE_URL}/healthz`;
    const response = await fetch(url);
    return response.ok;
  } catch (error) {
    console.error('Health check failed:', error);
    return false;
  }
}

