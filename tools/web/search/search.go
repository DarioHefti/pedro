// Package search implements a local metasearch engine: it queries several
// public search engines in parallel (no API keys, no AI search providers),
// merges their rankings with reciprocal rank fusion and removes duplicates.
package search

import (
	"context"
	"errors"
	"slices"
	"sort"
	"sync"
	"time"

	"pedro/tools/web/weburl"
)

// Result is a single search hit.
type Result struct {
	Title   string
	URL     string
	Snippet string
	Engines []string // engines that returned this URL (set by MetaSearcher)
}

// Provider is one search source.
type Provider interface {
	Name() string
	Search(ctx context.Context, query string) ([]Result, error)
}

// ErrBlocked is returned by a provider whose results page was a bot check.
var ErrBlocked = errors.New("blocked by bot protection")

// EngineStatus reports how one provider did for a query.
type EngineStatus struct {
	Name    string
	Results int
	Err     error
	Skipped bool // cooling down after recent failures
}

// Weighted pairs a provider with its influence on the merged ranking.
type Weighted struct {
	Provider Provider
	Weight   float64
}

// MetaSearcher fans a query out to all providers and fuses the results.
// Providers that hit a bot check are paused for cooldown.
type MetaSearcher struct {
	providers []Weighted
	timeout   time.Duration
	cooldown  time.Duration
	// Once quorum providers answered, stragglers get grace more time.
	quorum int
	grace  time.Duration

	mu           sync.Mutex
	coolingUntil map[string]time.Time
}

// NewMetaSearcher builds a MetaSearcher. timeout bounds the whole fan-out;
// a provider that fails is skipped for cooldown so it isn't hammered.
func NewMetaSearcher(timeout, cooldown time.Duration, providers ...Weighted) *MetaSearcher {
	return &MetaSearcher{
		providers:    providers,
		timeout:      timeout,
		cooldown:     cooldown,
		quorum:       2,
		grace:        3 * time.Second,
		coolingUntil: make(map[string]time.Time),
	}
}

// ErrTooSlow marks a provider that had not answered when the others were done.
var ErrTooSlow = errors.New("did not answer in time")

// Search queries every provider in parallel and returns at most limit merged results.
func (m *MetaSearcher) Search(ctx context.Context, query string, limit int) ([]Result, []EngineStatus) {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	type answer struct {
		idx     int
		results []Result
		err     error
	}
	statuses := make([]EngineStatus, len(m.providers))
	answers := make(chan answer, len(m.providers))
	pending := 0
	for i, wp := range m.providers {
		name := wp.Provider.Name()
		statuses[i] = EngineStatus{Name: name}
		if m.isCooling(name) {
			statuses[i].Skipped = true
			continue
		}
		pending++
		go func(i int, p Provider) {
			res, err := p.Search(ctx, query)
			answers <- answer{idx: i, results: res, err: err}
		}(i, wp.Provider)
	}

	perEngine := make([][]Result, len(m.providers))
	answered := make([]bool, len(m.providers))
	succeeded := 0
	var grace <-chan time.Time
wait:
	for ; pending > 0; pending-- {
		select {
		case a := <-answers:
			answered[a.idx] = true
			perEngine[a.idx] = a.results
			statuses[a.idx].Results = len(a.results)
			statuses[a.idx].Err = a.err
			if errors.Is(a.err, ErrBlocked) {
				m.cool(m.providers[a.idx].Provider.Name())
			}
			if a.err == nil {
				if succeeded++; succeeded == m.quorum {
					grace = time.After(m.grace)
				}
			}
		case <-grace:
			break wait
		case <-ctx.Done():
			break wait
		}
	}
	for i := range statuses {
		if !statuses[i].Skipped && !answered[i] {
			statuses[i].Err = ErrTooSlow
		}
	}

	return fuse(m.providers, perEngine, limit), statuses
}

func (m *MetaSearcher) isCooling(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Now().Before(m.coolingUntil[name])
}

func (m *MetaSearcher) cool(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.coolingUntil[name] = time.Now().Add(m.cooldown)
}

// rrfK dampens the advantage of top ranks (standard value from the RRF paper).
const rrfK = 60

// fuse merges per-engine rankings with weighted reciprocal rank fusion:
// a URL ranked well by several engines beats one ranked first by a single engine.
func fuse(providers []Weighted, perEngine [][]Result, limit int) []Result {
	type entry struct {
		result Result
		score  float64
		first  int // insertion order, for stable ties
	}
	byKey := make(map[string]*entry)
	order := 0
	for i, results := range perEngine {
		weight := providers[i].Weight
		name := providers[i].Provider.Name()
		for rank, r := range results {
			key := weburl.Key(r.URL)
			if key == "" {
				continue
			}
			e, ok := byKey[key]
			if !ok {
				e = &entry{result: Result{Title: r.Title, URL: r.URL, Snippet: r.Snippet}, first: order}
				order++
				byKey[key] = e
			} else if slices.Contains(e.result.Engines, name) {
				continue
			}
			e.score += weight / float64(rrfK+rank+1)
			e.result.Engines = append(e.result.Engines, name)
			if len(r.Snippet) > len(e.result.Snippet) {
				e.result.Snippet = r.Snippet
			}
			if e.result.Title == "" {
				e.result.Title = r.Title
			}
		}
	}

	entries := make([]*entry, 0, len(byKey))
	for _, e := range byKey {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].score != entries[j].score {
			return entries[i].score > entries[j].score
		}
		return entries[i].first < entries[j].first
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]Result, len(entries))
	for i, e := range entries {
		out[i] = e.result
	}
	return out
}

// Searcher runs a query against a set of providers.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]Result, []EngineStatus)
}
