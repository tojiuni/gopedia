package phloem

import "sync"

var (
	pipelines   = make(map[string]Pipeline)
	registryMu  sync.RWMutex
	defaultDomain = "wiki"
)

// Register adds a pipeline for the given domain. Safe for concurrent use.
func Register(domain string, p Pipeline) {
	registryMu.Lock()
	defer registryMu.Unlock()
	pipelines[domain] = p
}

// Get returns the pipeline for the domain. If not found, returns (nil, false).
func Get(domain string) (Pipeline, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := pipelines[domain]
	return p, ok
}

// GetOrDefault returns the pipeline for the domain, or the default domain pipeline if not found.
func GetOrDefault(domain string) Pipeline {
	if p, ok := Get(domain); ok {
		return p
	}
	p, _ := Get(defaultDomain)
	return p
}

// SetDefaultDomain sets the domain used when the request domain is missing or unknown.
func SetDefaultDomain(domain string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	defaultDomain = domain
}
