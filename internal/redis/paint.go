package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const paintScript = `
-- KEYS[1]=k_bits, KEYS[2]=k_seq
-- ARGV[1]=o, ARGV[2]=color, ARGV[3]=nowTs

local o = tonumber(ARGV[1])
local color = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local byteIdx = math.floor((o * 4) / 8)
local nibbleIsHigh = (o % 2) == 0

local cur = redis.call('GETRANGE', KEYS[1], byteIdx, byteIdx)
if cur == false or #cur == 0 then
  -- initialize 32 KiB if absent
  redis.call('SETRANGE', KEYS[1], 32767, string.char(0))
  cur = string.char(0)
end

local b = string.byte(cur)
local prev
if nibbleIsHigh then
  prev = bit.rshift(bit.band(b, 0xF0), 4)
  b = bit.bor(bit.band(b, 0x0F), bit.lshift(color, 4))
else
  prev = bit.band(b, 0x0F)
  b = bit.bor(bit.band(b, 0xF0), color)
end

redis.call('SETRANGE', KEYS[1], byteIdx, string.char(b))
local seq = redis.call('INCR', KEYS[2])

return { seq, now, prev }
`

// Client wraps a Redis client with paint-specific methods
type Client struct {
	client      *redis.Client
	ctx         context.Context
	paintScript *redis.Script
}

// NewClient creates a new Redis client
func NewClient(redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	script := redis.NewScript(paintScript)

	return &Client{
		client:      client,
		ctx:         context.Background(),
		paintScript: script,
	}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.client.Close()
}

// PaintTile atomically paints a tile and returns the new sequence number, timestamp, and previous color
func (c *Client) PaintTile(cx, cy int64, offset int, color uint8) (uint64, int64, uint8, error) {
	kBits := fmt.Sprintf("chunk:%d:%d:bits", cx, cy)
	kSeq := fmt.Sprintf("chunk:%d:%d:seq", cx, cy)

	result, err := c.paintScript.Run(c.ctx, c.client, []string{kBits, kSeq}, offset, color, time.Now().Unix()).Result()
	if err != nil {
		return 0, 0, 0, err
	}

	arr := result.([]interface{})
	seq := uint64(arr[0].(int64))
	ts := arr[1].(int64)
	prev := uint8(arr[2].(int64))

	return seq, ts, prev, nil
}

// GetChunkBits retrieves the full 32KB chunk bitstring
func (c *Client) GetChunkBits(cx, cy int64) ([]byte, error) {
	kBits := fmt.Sprintf("chunk:%d:%d:bits", cx, cy)
	return c.client.GetRange(c.ctx, kBits, 0, 32767).Bytes()
}

// GetChunkSeq retrieves the current sequence number for a chunk
func (c *Client) GetChunkSeq(cx, cy int64) (uint64, error) {
	kSeq := fmt.Sprintf("chunk:%d:%d:seq", cx, cy)
	return c.client.Get(c.ctx, kSeq).Uint64()
}

// SetCooldown sets a cooldown for an IP address
func (c *Client) SetCooldown(ip string, duration time.Duration) error {
	key := fmt.Sprintf("cool:%s", ip)
	return c.client.Set(c.ctx, key, time.Now().Unix(), duration).Err()
}

// CheckCooldown checks if an IP address is in cooldown
func (c *Client) CheckCooldown(ip string) (bool, error) {
	key := fmt.Sprintf("cool:%s", ip)
	exists, err := c.client.Exists(c.ctx, key).Result()
	return exists > 0, err
}

// FlushDB flushes the database (for testing only)
func (c *Client) FlushDB() error {
	return c.client.FlushDB(c.ctx).Err()
}

// Ping checks the Redis connection
func (c *Client) Ping() error {
	return c.client.Ping(c.ctx).Err()
}
