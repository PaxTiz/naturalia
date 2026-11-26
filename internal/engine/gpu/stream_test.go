package gpu

import (
	"testing"
	"time"
)

// The workers build every piece asked, each once, and Take hands them
// over without waiting; the GPU side (Upload) needs a window.
func TestStreamBuilds(t *testing.T) {
	s := NewStream(nil, func(k int) Data { return Data{Verts: []float32{float32(k), 0, 0}} })
	const n = 500
	for k := range n {
		s.Request(k)
	}
	if s.Pending() != n {
		t.Fatalf("pending %d, want %d", s.Pending(), n)
	}
	seen := map[int]bool{}
	deadline := time.Now().Add(10 * time.Second)
	for len(seen) < n && time.Now().Before(deadline) {
		for _, b := range s.Take(64) {
			if seen[b.Key] {
				t.Fatalf("piece %d built twice", b.Key)
			}
			if b.Verts[0] != float32(b.Key) {
				t.Fatalf("piece %d holds the data of %v", b.Key, b.Verts[0])
			}
			seen[b.Key] = true
		}
		time.Sleep(time.Millisecond)
	}
	if len(seen) != n || s.Pending() != 0 {
		t.Fatalf("%d pieces built, %d pending", len(seen), s.Pending())
	}
	if got := s.Take(8); len(got) != 0 {
		t.Fatalf("Take gave %d pieces more", len(got))
	}
}
