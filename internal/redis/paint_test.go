package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

// Test Redis operations and Lua scripts for the paint system

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

type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisClient() *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test database
	})

	return &RedisClient{
		client: client,
		ctx:    context.Background(),
	}
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

func (r *RedisClient) FlushDB() error {
	return r.client.FlushDB(r.ctx).Err()
}

func (r *RedisClient) PaintTile(cx, cy int64, offset int, color uint8) (uint64, int64, uint8, error) {
	kBits := fmt.Sprintf("chunk:%d:%d:bits", cx, cy)
	kSeq := fmt.Sprintf("chunk:%d:%d:seq", cx, cy)

	result, err := r.client.Eval(r.ctx, paintScript, []string{kBits, kSeq}, offset, color, time.Now().Unix()).Result()
	if err != nil {
		return 0, 0, 0, err
	}

	arr := result.([]interface{})
	seq := uint64(arr[0].(int64))
	ts := arr[1].(int64)
	prev := uint8(arr[2].(int64))

	return seq, ts, prev, nil
}

func (r *RedisClient) GetChunkBits(cx, cy int64) ([]byte, error) {
	kBits := fmt.Sprintf("chunk:%d:%d:bits", cx, cy)
	return r.client.GetRange(r.ctx, kBits, 0, 32767).Bytes()
}

func (r *RedisClient) GetChunkSeq(cx, cy int64) (uint64, error) {
	kSeq := fmt.Sprintf("chunk:%d:%d:seq", cx, cy)
	return r.client.Get(r.ctx, kSeq).Uint64()
}

func (r *RedisClient) SetCooldown(ip string, duration time.Duration) error {
	key := fmt.Sprintf("cool:%s", ip)
	return r.client.Set(r.ctx, key, time.Now().Unix(), duration).Err()
}

func (r *RedisClient) CheckCooldown(ip string) (bool, error) {
	key := fmt.Sprintf("cool:%s", ip)
	exists, err := r.client.Exists(r.ctx, key).Result()
	return exists > 0, err
}

func TestRedisPaintScript(t *testing.T) {
	// Skip if Redis is not available
	client := NewRedisClient()
	defer client.Close()

	// Test connection
	if err := client.client.Ping(client.ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	// Clean up test database
	client.FlushDB()

	// Test painting a tile
	cx, cy := int64(0), int64(0)
	offset := 0
	color := uint8(5)

	seq, ts, prev, err := client.PaintTile(cx, cy, offset, color)
	if err != nil {
		t.Fatalf("PaintTile failed: %v", err)
	}

	// Verify sequence number
	if seq != 1 {
		t.Errorf("Expected sequence 1, got %d", seq)
	}

	// Verify previous color (should be 0 for new tile)
	if prev != 0 {
		t.Errorf("Expected previous color 0, got %d", prev)
	}

	// Verify timestamp is recent
	now := time.Now().Unix()
	if ts < now-5 || ts > now+5 {
		t.Errorf("Timestamp %d is not recent (now: %d)", ts, now)
	}

	// Test painting another tile in the same chunk
	offset2 := 1
	color2 := uint8(3)

	seq2, ts2, prev2, err := client.PaintTile(cx, cy, offset2, color2)
	if err != nil {
		t.Fatalf("Second PaintTile failed: %v", err)
	}

	// Verify sequence incremented
	if seq2 != seq+1 {
		t.Errorf("Expected sequence %d, got %d", seq+1, seq2)
	}

	// Verify previous color for new tile
	if prev2 != 0 {
		t.Errorf("Expected previous color 0 for new tile, got %d", prev2)
	}
}

func TestRedisPaintOverwrite(t *testing.T) {
	client := NewRedisClient()
	defer client.Close()

	if err := client.client.Ping(client.ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.FlushDB()

	cx, cy := int64(0), int64(0)
	offset := 0
	originalColor := uint8(5)
	newColor := uint8(3)

	// Paint original color
	_, _, prev1, err := client.PaintTile(cx, cy, offset, originalColor)
	if err != nil {
		t.Fatalf("First PaintTile failed: %v", err)
	}

	// Verify previous color was 0
	if prev1 != 0 {
		t.Errorf("Expected previous color 0, got %d", prev1)
	}

	// Overwrite with new color
	_, _, prev2, err := client.PaintTile(cx, cy, offset, newColor)
	if err != nil {
		t.Fatalf("Second PaintTile failed: %v", err)
	}

	// Verify previous color was the original
	if prev2 != originalColor {
		t.Errorf("Expected previous color %d, got %d", originalColor, prev2)
	}
}

func TestRedisChunkInitialization(t *testing.T) {
	client := NewRedisClient()
	defer client.Close()

	if err := client.client.Ping(client.ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.FlushDB()

	cx, cy := int64(0), int64(0)

	// Get chunk bits before any painting
	bits, err := client.GetChunkBits(cx, cy)
	if err != nil {
		t.Fatalf("GetChunkBits failed: %v", err)
	}

	// Should be empty or all zeros
	if len(bits) > 0 {
		// Check if all bytes are zero
		allZero := true
		for _, b := range bits {
			if b != 0 {
				allZero = false
				break
			}
		}
		if !allZero {
			t.Errorf("Chunk bits should be all zeros, got: %v", bits)
		}
	}

	// Paint a tile to initialize the chunk
	_, _, _, err = client.PaintTile(cx, cy, 0, 5)
	if err != nil {
		t.Fatalf("PaintTile failed: %v", err)
	}

	// Now get the chunk bits again
	bits2, err := client.GetChunkBits(cx, cy)
	if err != nil {
		t.Fatalf("GetChunkBits after paint failed: %v", err)
	}

	// Should now have data
	if len(bits2) == 0 {
		t.Errorf("Chunk bits should be initialized after painting")
	}

	// Should be 32KB (32768 bytes)
	if len(bits2) != 32768 {
		t.Errorf("Expected 32768 bytes, got %d", len(bits2))
	}
}

func TestRedisSequenceIncrement(t *testing.T) {
	client := NewRedisClient()
	defer client.Close()

	if err := client.client.Ping(client.ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.FlushDB()

	cx, cy := int64(0), int64(0)

	// Paint multiple tiles and verify sequence increments
	expectedSeq := uint64(1)
	for i := 0; i < 10; i++ {
		seq, _, _, err := client.PaintTile(cx, cy, i, uint8(i%16))
		if err != nil {
			t.Fatalf("PaintTile %d failed: %v", i, err)
		}

		if seq != expectedSeq {
			t.Errorf("Expected sequence %d, got %d", expectedSeq, seq)
		}
		expectedSeq++
	}

	// Verify final sequence number
	finalSeq, err := client.GetChunkSeq(cx, cy)
	if err != nil {
		t.Fatalf("GetChunkSeq failed: %v", err)
	}

	if finalSeq != 10 {
		t.Errorf("Expected final sequence 10, got %d", finalSeq)
	}
}

func TestRedisCooldown(t *testing.T) {
	client := NewRedisClient()
	defer client.Close()

	if err := client.client.Ping(client.ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.FlushDB()

	ip := "192.168.1.1"
	duration := 5 * time.Second

	// Initially no cooldown
	hasCooldown, err := client.CheckCooldown(ip)
	if err != nil {
		t.Fatalf("CheckCooldown failed: %v", err)
	}
	if hasCooldown {
		t.Errorf("Should not have cooldown initially")
	}

	// Set cooldown
	err = client.SetCooldown(ip, duration)
	if err != nil {
		t.Fatalf("SetCooldown failed: %v", err)
	}

	// Check cooldown exists
	hasCooldown, err = client.CheckCooldown(ip)
	if err != nil {
		t.Fatalf("CheckCooldown after set failed: %v", err)
	}
	if !hasCooldown {
		t.Errorf("Should have cooldown after setting")
	}

	// Wait for cooldown to expire
	time.Sleep(duration + 100*time.Millisecond)

	// Check cooldown expired
	hasCooldown, err = client.CheckCooldown(ip)
	if err != nil {
		t.Fatalf("CheckCooldown after expiry failed: %v", err)
	}
	if hasCooldown {
		t.Errorf("Should not have cooldown after expiry")
	}
}

func TestRedisMultipleChunks(t *testing.T) {
	client := NewRedisClient()
	defer client.Close()

	if err := client.client.Ping(client.ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.FlushDB()

	// Test multiple chunks
	chunks := []struct {
		cx, cy int64
	}{
		{0, 0},
		{1, 0},
		{0, 1},
		{1, 1},
	}

	for i, chunk := range chunks {
		seq, _, _, err := client.PaintTile(chunk.cx, chunk.cy, 0, uint8(i+1))
		if err != nil {
			t.Fatalf("PaintTile for chunk (%d, %d) failed: %v", chunk.cx, chunk.cy, err)
		}

		// Each chunk should start with sequence 1
		if seq != 1 {
			t.Errorf("Chunk (%d, %d) should start with sequence 1, got %d", chunk.cx, chunk.cy, seq)
		}
	}

	// Verify each chunk has its own sequence
	for _, chunk := range chunks {
		seq, err := client.GetChunkSeq(chunk.cx, chunk.cy)
		if err != nil {
			t.Fatalf("GetChunkSeq for chunk (%d, %d) failed: %v", chunk.cx, chunk.cy, err)
		}

		if seq != 1 {
			t.Errorf("Chunk (%d, %d) should have sequence 1, got %d", chunk.cx, chunk.cy, seq)
		}
	}
}

func TestRedisConcurrentPaints(t *testing.T) {
	client := NewRedisClient()
	defer client.Close()

	if err := client.client.Ping(client.ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	client.FlushDB()

	cx, cy := int64(0), int64(0)
	numGoroutines := 10
	paintsPerGoroutine := 5

	// Channel to collect results
	results := make(chan struct {
		seq   uint64
		error error
	}, numGoroutines*paintsPerGoroutine)

	// Launch concurrent painters
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < paintsPerGoroutine; j++ {
				offset := goroutineID*paintsPerGoroutine + j
				seq, _, _, err := client.PaintTile(cx, cy, offset, uint8(offset%16))
				results <- struct {
					seq   uint64
					error error
				}{seq, err}
			}
		}(i)
	}

	// Collect results
	sequences := make([]uint64, 0, numGoroutines*paintsPerGoroutine)
	for i := 0; i < numGoroutines*paintsPerGoroutine; i++ {
		result := <-results
		if result.error != nil {
			t.Errorf("Concurrent paint failed: %v", result.error)
		}
		sequences = append(sequences, result.seq)
	}

	// Verify all sequences are unique and sequential
	seqMap := make(map[uint64]bool)
	for _, seq := range sequences {
		if seqMap[seq] {
			t.Errorf("Duplicate sequence number: %d", seq)
		}
		seqMap[seq] = true
	}

	// Verify sequence range
	expectedMaxSeq := uint64(numGoroutines * paintsPerGoroutine)
	for _, seq := range sequences {
		if seq < 1 || seq > expectedMaxSeq {
			t.Errorf("Sequence %d out of range [1, %d]", seq, expectedMaxSeq)
		}
	}
}

func BenchmarkRedisPaint(t *testing.B) {
	client := NewRedisClient()
	defer client.Close()

	if err := client.client.Ping(client.ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping benchmark")
	}

	client.FlushDB()

	cx, cy := int64(0), int64(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		offset := i % 65536
		color := uint8(i % 16)
		client.PaintTile(cx, cy, offset, color)
	}
}
