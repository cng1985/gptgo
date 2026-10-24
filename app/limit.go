package app

import (
	"sync"
	"time"
)

// sliding 按时间窗口限制同一键的尝试次数。
type sliding struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func newSliding(limit int, window time.Duration) *sliding {
	return &sliding{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
	}
}

func (s *sliding) allow(key string) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.hits) > 4096 {
		s.hits = make(map[string][]time.Time)
	}

	cutoff := now.Add(-s.window)
	prev := s.hits[key]
	kept := make([]time.Time, 0, len(prev)+1)
	for _, ts := range prev {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= s.limit {
		s.hits[key] = kept
		return false
	}
	s.hits[key] = append(kept, now)
	return true
}
