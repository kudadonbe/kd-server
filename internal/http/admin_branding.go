package http

import (
	"strings"
	"sync"
)

// brandingState holds the operator-console display name and environment badge.
// It is seeded from configuration (KD_SERVER_NAME / KD_ENV) and can be edited
// live from Settings › Branding. The change is in-memory only: it distinguishes
// deployments (dev/stage/prod) for the person looking at the console right now,
// and resets to the configured values on restart. It is safe for concurrent use.
type brandingState struct {
	mu   sync.RWMutex
	name string
	env  string
}

func newBrandingState(name, env string) *brandingState {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "KD-Server"
	}
	env = strings.TrimSpace(env)
	if env == "" {
		env = "dev"
	}
	return &brandingState{name: name, env: env}
}

// get returns the current display name and environment badge.
func (b *brandingState) get() (name, env string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.name, b.env
}

// set updates the branding. Blank fields are ignored so a partial form submit
// never wipes an existing value.
func (b *brandingState) set(name, env string) {
	name = strings.TrimSpace(name)
	env = strings.TrimSpace(env)
	b.mu.Lock()
	defer b.mu.Unlock()
	if name != "" {
		b.name = name
	}
	if env != "" {
		b.env = env
	}
}
