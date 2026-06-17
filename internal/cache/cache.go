// Package cache is a tiny, TTL-based JSON cache on local disk. It is used for
// slow-changing metadata (workspaces, boards, members, columns) so repeated
// commands don't re-fetch the same data from the API.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Store is a directory of cached JSON entries.
type Store struct{ dir string }

// New returns a Store rooted at dir (created lazily on first write).
func New(dir string) *Store { return &Store{dir: dir} }

type envelope struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Key       string          `json:"key"`
	Data      json.RawMessage `json:"data"`
}

func (s *Store) path(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(h[:10])+".json")
}

// Get decodes a fresh (within maxAge) cached value into out, reporting whether
// it was a hit.
func (s *Store) Get(key string, maxAge time.Duration, out any) bool {
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return false
	}
	var e envelope
	if json.Unmarshal(data, &e) != nil {
		return false
	}
	if time.Since(e.FetchedAt) > maxAge {
		return false
	}
	return json.Unmarshal(e.Data, out) == nil
}

// Put stores v under key.
func (s *Store) Put(key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(envelope{FetchedAt: time.Now(), Key: key, Data: raw}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path(key), b, 0o644)
}

// Delete removes a cached entry (used to invalidate after writes).
func (s *Store) Delete(key string) { _ = os.Remove(s.path(key)) }

// Clear removes the whole cache directory.
func (s *Store) Clear() error { return os.RemoveAll(s.dir) }
