package http

import (
	"net/http"

	"github.com/a-h/templ"
)

// writeFragments renders one or more templ components in sequence as a single
// HTMX response (e.g. a target swap plus out-of-band updates).
func writeFragments(w http.ResponseWriter, r *http.Request, cs ...templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, c := range cs {
		_ = c.Render(r.Context(), w)
	}
}

// adminHXHandler serves the console's HTMX fragment endpoints under /admin/hx/.
// It returns HTML fragments (not JSON) and, on an expired session, an HX-Redirect
// back to /admin. The JSON /admin/api/* endpoints are left untouched.
func adminHXHandler(cfg Config, adminAuth *adminAuthenticator, branding *brandingState) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/admin/hx/login" && r.Method == http.MethodPost:
			_ = r.ParseForm()
			if err := adminAuth.login(w, r, r.PostFormValue("username"), r.PostFormValue("password")); err != nil {
				writeFragment(w, r, Message(err.Error(), false))
				return
			}
			w.Header().Set("HX-Redirect", "/admin")
			w.WriteHeader(http.StatusOK)
			return
		case r.URL.Path == "/admin/hx/logout" && r.Method == http.MethodPost:
			adminAuth.logout(w, r)
			w.Header().Set("HX-Redirect", "/admin")
			w.WriteHeader(http.StatusOK)
			return
		}

		if !adminAuth.authenticated(r) {
			w.Header().Set("HX-Redirect", "/admin")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.URL.Path == "/admin/hx/tenants" && r.Method == http.MethodPost:
			_ = r.ParseForm()
			if _, err := cfg.AdminService.CreateTenant(r.Context(), r.PostFormValue("slug"), r.PostFormValue("name")); err != nil {
				writeFragment(w, r, Message(err.Error(), false))
				return
			}
			tenants, _ := cfg.AdminService.ListTenants(r.Context())
			writeFragment(w, r, TenantCreated(tenants, "Tenant created."))

		case r.URL.Path == "/admin/hx/keys" && r.Method == http.MethodPost:
			_ = r.ParseForm()
			issued, err := cfg.AdminService.IssueAPIKey(r.Context(), r.PostFormValue("tenant"), r.PostFormValue("label"))
			if err != nil {
				writeFragment(w, r, Message(err.Error(), false))
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeFragment(w, r, IssuedKey(issued.Secret, issued.KeyID))

		case r.URL.Path == "/admin/hx/keys/revoke" && r.Method == http.MethodPost:
			_ = r.ParseForm()
			if err := cfg.AdminService.RevokeAPIKey(r.Context(), r.PostFormValue("tenant"), r.PostFormValue("key_id")); err != nil {
				writeFragment(w, r, Message(err.Error(), false))
				return
			}
			writeFragment(w, r, Message("API key revoked.", true))

		case r.URL.Path == "/admin/hx/ai-credentials" && r.Method == http.MethodPost && cfg.AIService != nil:
			_ = r.ParseForm()
			if _, err := cfg.AIService.SetDefault(r.Context(), r.PostFormValue("api_key"), r.PostFormValue("model")); err != nil {
				writeFragment(w, r, Message(aiErrMessage(err), false))
				return
			}
			status, _ := cfg.AIService.DefaultStatus(r.Context())
			writeFragments(w, r, AIStatus(status, true), topbarOOB(shellDataFor(r, cfg, branding)))

		case r.URL.Path == "/admin/hx/ai-credentials" && r.Method == http.MethodDelete && cfg.AIService != nil:
			_ = cfg.AIService.DeleteDefault(r.Context())
			status, _ := cfg.AIService.DefaultStatus(r.Context())
			writeFragments(w, r, AIStatus(status, true), topbarOOB(shellDataFor(r, cfg, branding)))

		case r.URL.Path == "/admin/hx/ai-validate" && r.Method == http.MethodPost && cfg.AIService != nil:
			model, err := cfg.AIService.ValidateDefault(r.Context())
			if err != nil {
				writeFragment(w, r, Message(aiErrMessage(err), false))
				return
			}
			writeFragment(w, r, Message("Key OK — validated "+model+".", true))

		case r.URL.Path == "/admin/hx/branding" && r.Method == http.MethodPost:
			_ = r.ParseForm()
			branding.set(r.PostFormValue("name"), r.PostFormValue("env"))
			writeFragments(w, r, Message("Branding updated.", true), topbarOOB(shellDataFor(r, cfg, branding)))

		case r.URL.Path == "/admin/hx/logout-all" && r.Method == http.MethodPost:
			adminAuth.logoutAll()
			w.Header().Set("HX-Redirect", "/admin")
			w.WriteHeader(http.StatusOK)

		default:
			writeFragment(w, r, Message("Not found.", false))
		}
	})
}
