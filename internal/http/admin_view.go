package http

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"

	"github.com/kudadonbe/kd-server/internal/ai"
)

// adminConsoleCSP is tightened for the templ console: htmx and the stylesheet
// are served from the same origin, so no inline scripts are allowed.
const adminConsoleCSP = "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; base-uri 'none'; frame-ancestors 'none'"

// renderAdmin writes a full templ admin page with the console CSP.
func renderAdmin(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", adminConsoleCSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = c.Render(r.Context(), w)
}

// writeFragment renders a templ fragment for an HTMX swap.
func writeFragment(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = c.Render(r.Context(), w)
}

// aiErrMessage maps an AI error to a user-safe message for the console.
func aiErrMessage(err error) string {
	switch {
	case errors.Is(err, ai.ErrInvalidKey):
		return "API key rejected by provider."
	case errors.Is(err, ai.ErrStorageDisabled):
		return "Set AI_ENCRYPTION_KEY to store a key in the UI (the .env ANTHROPIC_API_KEY still works)."
	case errors.Is(err, ai.ErrNotConfigured):
		return "No default key configured (set ANTHROPIC_API_KEY or pair one)."
	default:
		return "Could not reach the provider."
	}
}
