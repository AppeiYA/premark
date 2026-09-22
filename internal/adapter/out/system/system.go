package system

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"premark/internal/ports"
)

var _ ports.Clock = (*SystemClock)(nil)
var _ ports.IDGenerator = (*RandomIDGenerator)(nil)

type SystemClock struct{}

func NewClock() *SystemClock {
	return &SystemClock{}
}

func (c *SystemClock) Now() time.Time {
	return time.Now().UTC()
}

type RandomIDGenerator struct{}

func NewIDGenerator() *RandomIDGenerator {
	return &RandomIDGenerator{}
}

func (g *RandomIDGenerator) NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
