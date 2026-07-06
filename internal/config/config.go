package config

import (
	"sync"
	"time"
)

// Config é o estado vivo e mutável do bot. Thread-safe.
type Config struct {
	mu sync.RWMutex

	Persona         string
	VADThreshold    float64
	MentionCooldown time.Duration
	SpontCooldown   time.Duration
	AutoCooldown    time.Duration
	Vision          bool
	SpontChance     float64
}

func Default(persona string) *Config {
	return &Config{
		Persona:         persona,
		VADThreshold:    500,
		MentionCooldown: 45 * time.Second,
		SpontCooldown:   150 * time.Second,
		AutoCooldown:    90 * time.Second,
		Vision:          true,
		SpontChance:     0.6,
	}
}

func (c *Config) Snapshot() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Config{
		Persona:         c.Persona,
		VADThreshold:    c.VADThreshold,
		MentionCooldown: c.MentionCooldown,
		SpontCooldown:   c.SpontCooldown,
		AutoCooldown:    c.AutoCooldown,
		Vision:          c.Vision,
		SpontChance:     c.SpontChance,
	}
}

func (c *Config) Update(fn func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c)
}
