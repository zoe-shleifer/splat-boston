// WebSocket client for real-time delta updates

const WS_BASE_URL = process.env.REACT_APP_WS_URL || 'ws://localhost:8080';

export interface Delta {
  seq: number;
  o: number;
  color: number;
  ts: number;
}

export type DeltaCallback = (delta: Delta) => void;
export type ErrorCallback = (error: Event) => void;
export type CloseCallback = () => void;
export type OpenCallback = () => void;

/**
 * WebSocket client for subscribing to chunk updates
 */
export class ChunkWebSocket {
  private ws: WebSocket | null = null;
  private cx: number;
  private cy: number;
  private onDelta: DeltaCallback;
  private onError?: ErrorCallback;
  private onClose?: CloseCallback;
  private onOpen?: OpenCallback;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private shouldReconnect = true;

  constructor(
    cx: number,
    cy: number,
    onDelta: DeltaCallback,
    onError?: ErrorCallback,
    onClose?: CloseCallback,
    onOpen?: OpenCallback
  ) {
    this.cx = cx;
    this.cy = cy;
    this.onDelta = onDelta;
    this.onError = onError;
    this.onClose = onClose;
    this.onOpen = onOpen;
  }

  /**
   * Connect to the WebSocket
   */
  connect(): void {
    const url = `${WS_BASE_URL}/sub?cx=${this.cx}&cy=${this.cy}`;
    
    try {
      this.ws = new WebSocket(url);
      
      this.ws.onopen = () => {
        console.log(`WebSocket connected: chunk (${this.cx}, ${this.cy})`);
        this.reconnectAttempts = 0;
        if (this.onOpen) {
          this.onOpen();
        }
      };
      
      this.ws.onmessage = (event) => {
        try {
          const delta: Delta = JSON.parse(event.data);
          this.onDelta(delta);
        } catch (error) {
          console.error('Failed to parse delta:', error);
        }
      };
      
      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        if (this.onError) {
          this.onError(error);
        }
      };
      
      this.ws.onclose = () => {
        console.log(`WebSocket closed: chunk (${this.cx}, ${this.cy})`);
        if (this.onClose) {
          this.onClose();
        }
        
        // Attempt to reconnect
        if (this.shouldReconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++;
          const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
          console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
          setTimeout(() => this.connect(), delay);
        }
      };
    } catch (error) {
      console.error('Failed to create WebSocket:', error);
    }
  }

  /**
   * Disconnect from the WebSocket
   */
  disconnect(): void {
    this.shouldReconnect = false;
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  /**
   * Check if the WebSocket is connected
   */
  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }
}

/**
 * Manager for multiple chunk WebSocket connections
 */
export class ChunkWebSocketManager {
  private connections = new Map<string, ChunkWebSocket>();

  /**
   * Subscribe to a chunk
   */
  subscribe(
    cx: number,
    cy: number,
    onDelta: DeltaCallback,
    onError?: ErrorCallback,
    onClose?: CloseCallback,
    onOpen?: OpenCallback
  ): void {
    const key = `${cx}:${cy}`;
    
    // If already subscribed, disconnect first
    if (this.connections.has(key)) {
      this.unsubscribe(cx, cy);
    }
    
    const ws = new ChunkWebSocket(cx, cy, onDelta, onError, onClose, onOpen);
    ws.connect();
    this.connections.set(key, ws);
  }

  /**
   * Unsubscribe from a chunk
   */
  unsubscribe(cx: number, cy: number): void {
    const key = `${cx}:${cy}`;
    const ws = this.connections.get(key);
    
    if (ws) {
      ws.disconnect();
      this.connections.delete(key);
    }
  }

  /**
   * Unsubscribe from all chunks
   */
  unsubscribeAll(): void {
    for (const ws of this.connections.values()) {
      ws.disconnect();
    }
    this.connections.clear();
  }

  /**
   * Get the connection for a chunk
   */
  getConnection(cx: number, cy: number): ChunkWebSocket | undefined {
    const key = `${cx}:${cy}`;
    return this.connections.get(key);
  }
}

