package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"github.com/kudadonbe/kd-server/internal/ai"
	"github.com/kudadonbe/kd-server/internal/services"
)

// comp aliases templ.Component so the .templ files can accept an already-rendered
// panel as a parameter without importing the templ runtime themselves (the
// generator imports it for them; a second explicit import would clash).
type comp = templ.Component

// shellData is the console chrome shared by every authenticated page: the
// branding shown in the sidebar/topbar and the current AI pairing for the pill.
type shellData struct {
	ServerName string
	Env        string
	AIEnabled  bool
	AIPaired   bool
}

// tileVM is a single Overview stat tile. A non-empty Href turns it into an
// HTMX nav link to that section.
type tileVM struct {
	Label  string
	Value  string
	Href   string
	Action string
}

// settingsData drives the Settings panel (AI, server info, security, branding).
type settingsData struct {
	ServerName     string
	Env            string
	Version        string
	AIEnabled      bool
	AIStatus       ai.DefaultStatus
	StorageEnabled bool
	Sessions       int
}

// knownSections are the top-level console panels served under /admin.
var knownSections = map[string]bool{
	"overview": true,
	"tenants":  true,
	"keys":     true,
	"settings": true,
}

// adminConsoleHandler serves the templ operator console at /admin and
// /admin/{section}. A direct visit renders the full page (topbar + sidebar +
// panel); an HTMX nav (HX-Request) renders just the panel plus out-of-band
// sidebar/topbar updates so the active item and AI pill stay in sync.
func adminConsoleHandler(cfg Config, adminAuth *adminAuthenticator, branding *brandingState) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		section := strings.Trim(strings.TrimPrefix(r.URL.Path, "/admin"), "/")
		if section == "" {
			section = "overview"
		}
		if !knownSections[section] {
			http.NotFound(w, r)
			return
		}

		sd := shellDataFor(r, cfg, branding)
		isHX := r.Header.Get("HX-Request") == "true"

		if !adminAuth.authenticated(r) {
			if isHX {
				w.Header().Set("HX-Redirect", "/admin")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			renderAdmin(w, r, LoginPage(sd.ServerName))
			return
		}

		panel := sectionPanel(r, cfg, adminAuth, branding, section, sd)
		if isHX {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = panel.Render(r.Context(), w)
			_ = sidebarOOB(sd, section).Render(r.Context(), w)
			_ = topbarOOB(sd).Render(r.Context(), w)
			return
		}
		renderAdmin(w, r, Shell(sd, section, panel))
	})
}

// sectionPanel builds the templ component for a section, loading only the data
// that section needs.
func sectionPanel(r *http.Request, cfg Config, adminAuth *adminAuthenticator, branding *brandingState, section string, sd shellData) comp {
	ctx := r.Context()
	switch section {
	case "tenants":
		return TenantsPanel(listTenants(ctx, cfg))
	case "keys":
		return KeysPanel(listTenants(ctx, cfg))
	case "settings":
		return SettingsPanel(settingsDataFor(r, cfg, adminAuth, branding, sd))
	default:
		return OverviewPanel(sd, overviewTiles(ctx, cfg, sd))
	}
}

func listTenants(ctx context.Context, cfg Config) []services.TenantSummary {
	if cfg.AdminService == nil {
		return nil
	}
	tenants, _ := cfg.AdminService.ListTenants(ctx)
	return tenants
}

func shellDataFor(r *http.Request, cfg Config, branding *brandingState) shellData {
	name, env := branding.get()
	sd := shellData{ServerName: name, Env: env, AIEnabled: cfg.AIService != nil}
	if sd.AIEnabled {
		if st, err := cfg.AIService.DefaultStatus(r.Context()); err == nil {
			sd.AIPaired = st.Paired
		}
	}
	return sd
}

func overviewTiles(ctx context.Context, cfg Config, sd shellData) []tileVM {
	version := "—"
	if cfg.VersionService != nil {
		if v, err := cfg.VersionService.Version(ctx); err == nil {
			version = v
		}
	}
	aiValue := "off"
	if sd.AIEnabled {
		aiValue = "unpaired"
		if sd.AIPaired {
			aiValue = "paired"
		}
	}
	return []tileVM{
		{Label: "Tenants", Value: strconv.Itoa(len(listTenants(ctx, cfg))), Href: "/admin/tenants", Action: "Manage"},
		{Label: "AI provider", Value: aiValue, Href: "/admin/settings", Action: "Configure"},
		{Label: "Environment", Value: sd.Env},
		{Label: "Version", Value: version},
	}
}

func settingsDataFor(r *http.Request, cfg Config, adminAuth *adminAuthenticator, branding *brandingState, sd shellData) settingsData {
	d := settingsData{
		ServerName: sd.ServerName,
		Env:        sd.Env,
		Version:    "—",
		Sessions:   adminAuth.sessionCount(),
		AIEnabled:  cfg.AIService != nil,
	}
	if cfg.VersionService != nil {
		if v, err := cfg.VersionService.Version(r.Context()); err == nil {
			d.Version = v
		}
	}
	if d.AIEnabled {
		if st, err := cfg.AIService.DefaultStatus(r.Context()); err == nil {
			d.AIStatus = st
		}
		d.StorageEnabled = cfg.AIService.StorageEnabled()
	}
	return d
}

// navClass returns the sidebar link class, marking the active section.
func navClass(active, key string) string {
	if active == key {
		return "navlink active"
	}
	return "navlink"
}

// aiSourceText renders the human-readable AI provider line for Server info.
func aiSourceText(d settingsData) string {
	if !d.AIEnabled {
		return "not enabled (set ANTHROPIC_API_KEY)"
	}
	provider := d.AIStatus.Provider
	if provider == "" {
		provider = "anthropic"
	}
	switch d.AIStatus.Source {
	case "env":
		return provider + " · paired via ANTHROPIC_API_KEY"
	case "stored":
		return provider + " · paired (stored key)"
	default:
		return provider + " · enabled, not paired"
	}
}

func storageText(enabled bool) string {
	if enabled {
		return "encrypted storage on (AI_ENCRYPTION_KEY set)"
	}
	return "off — env key only (set AI_ENCRYPTION_KEY to store keys)"
}

func sessionsText(n int) string {
	if n == 1 {
		return "1 active session"
	}
	return strconv.Itoa(n) + " active sessions"
}
