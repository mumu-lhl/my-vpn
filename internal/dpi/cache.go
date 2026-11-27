package dpi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DomainCacheEntry stores the best known anti-DPI strategy for a specific domain.
type DomainCacheEntry struct {
	Domain        string `json:"domain"`
	StrategyID    string `json:"strategy_id"`
	LatencyMs     int64  `json:"latency_ms"`
	ECHSupported  bool   `json:"ech_supported"`
	FailureCount  int    `json:"failure_count"`
	Locked        bool   `json:"locked"`
	UpdatedAtUnix int64  `json:"updated_at_unix"`
}

// DomainStrategyCache persists per-domain optimal bypass strategies on disk
// and automatically evicts unlocked entries after repeated connection failures.
type DomainStrategyCache struct {
	mu      sync.RWMutex
	path    string
	entries map[string]DomainCacheEntry
}

func NewDomainStrategyCache(filePath string) *DomainStrategyCache {
	c := &DomainStrategyCache{
		path:    filePath,
		entries: make(map[string]DomainCacheEntry),
	}
	c.load()
	return c
}

func normalizeDomain(d string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
}

func (c *DomainStrategyCache) load() {
	if c.path == "" {
		return
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return
	}
	var list []DomainCacheEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}
	for _, ent := range list {
		key := normalizeDomain(ent.Domain)
		if key != "" && ent.StrategyID != "" {
			ent.Domain = key
			c.entries[key] = ent
		}
	}
}

func (c *DomainStrategyCache) saveLocked() error {
	if c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	list := c.listLocked()
	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

func (c *DomainStrategyCache) listLocked() []DomainCacheEntry {
	out := make([]DomainCacheEntry, 0, len(c.entries))
	for _, ent := range c.entries {
		out = append(out, ent)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Locked != out[j].Locked {
			return out[i].Locked
		}
		return out[i].Domain < out[j].Domain
	})
	return out
}

func (c *DomainStrategyCache) Get(domain string) (DomainCacheEntry, bool) {
	key := normalizeDomain(domain)
	if key == "" {
		return DomainCacheEntry{}, false
	}
	c.mu.RLock()
	ent, ok := c.entries[key]
	c.mu.RUnlock()
	return ent, ok
}

func (c *DomainStrategyCache) RecordSuccess(domain, strategyID string, latencyMs int64, echSupported bool) error {
	key := normalizeDomain(domain)
	if key == "" || strategyID == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	prev := c.entries[key]
	if prev.Locked && prev.StrategyID != strategyID {
		return nil
	}
	c.entries[key] = DomainCacheEntry{
		Domain:        key,
		StrategyID:    strategyID,
		LatencyMs:     latencyMs,
		ECHSupported:  echSupported,
		FailureCount:  0,
		Locked:        prev.Locked,
		UpdatedAtUnix: time.Now().Unix(),
	}
	return c.saveLocked()
}

// RecordFailure increments the failure count for domain and evicts unlocked entries
// once consecutive failures reach 2 so the domain automatically falls back.
func (c *DomainStrategyCache) RecordFailure(domain string) (evicted bool, err error) {
	key := normalizeDomain(domain)
	if key == "" {
		return false, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.entries[key]
	if !ok || ent.Locked {
		return false, nil
	}
	ent.FailureCount++
	ent.UpdatedAtUnix = time.Now().Unix()
	if ent.FailureCount >= 2 {
		delete(c.entries, key)
		return true, c.saveLocked()
	}
	c.entries[key] = ent
	return false, c.saveLocked()
}

func (c *DomainStrategyCache) Lock(domain, strategyID string) error {
	key := normalizeDomain(domain)
	if key == "" || strategyID == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	prev := c.entries[key]
	c.entries[key] = DomainCacheEntry{
		Domain:        key,
		StrategyID:    strategyID,
		LatencyMs:     prev.LatencyMs,
		ECHSupported:  prev.ECHSupported,
		FailureCount:  0,
		Locked:        true,
		UpdatedAtUnix: time.Now().Unix(),
	}
	return c.saveLocked()
}

func (c *DomainStrategyCache) Delete(domain string) error {
	key := normalizeDomain(domain)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return c.saveLocked()
}

func (c *DomainStrategyCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]DomainCacheEntry)
	return c.saveLocked()
}

func (c *DomainStrategyCache) List() []DomainCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.listLocked()
}
