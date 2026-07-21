package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kudadonbe/kd-server/internal/ai"
	"github.com/kudadonbe/kd-server/internal/store"
)

const maxAdminDocumentUpload = 10 << 20

func landingHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, landingPage)
}

// identityDocumentPageHandler serves the identity-document extraction/record tool
// at /admin/identity. It is a focused testing tool (Tools › Identity docs); tenant,
// key, and AI management live in the templ console at /admin. It reuses the admin
// session (login here if you land unauthenticated) and the /admin/api/* endpoints.
func identityDocumentPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/admin/identity" {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, identityDocumentPage)
}

func adminAPIHandler(cfg Config, adminAuth *adminAuthenticator) http.Handler {
	protected := adminAuth.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/admin/api/tenants" && r.Method == http.MethodGet:
			tenants, err := cfg.AdminService.ListTenants(r.Context())
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list tenants failed"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"tenants": tenants})
		case r.URL.Path == "/admin/api/tenants" && r.Method == http.MethodPost:
			var req struct {
				Slug string `json:"slug"`
				Name string `json:"name"`
			}
			if err := decodeAdminJSON(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			tenant, err := cfg.AdminService.CreateTenant(r.Context(), req.Slug, req.Name)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusCreated, tenant)
		case r.URL.Path == "/admin/api/tenants/update" && r.Method == http.MethodPost:
			var req struct {
				Slug string `json:"slug"`
				Name string `json:"name"`
			}
			if err := decodeAdminJSON(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			tenant, err := cfg.AdminService.UpdateTenantName(r.Context(), req.Slug, req.Name)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, tenant)
		case r.URL.Path == "/admin/api/tenants/origins" && r.Method == http.MethodPost:
			var req struct {
				Slug           string   `json:"slug"`
				AllowedOrigins []string `json:"allowed_origins"`
			}
			if err := decodeAdminJSON(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			tenant, err := cfg.AdminService.SetTenantAllowedOrigins(r.Context(), req.Slug, req.AllowedOrigins)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, tenant)
		case r.URL.Path == "/admin/api/keys" && r.Method == http.MethodPost:
			var req struct {
				Tenant string `json:"tenant"`
				Label  string `json:"label"`
			}
			if err := decodeAdminJSON(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			issued, err := cfg.AdminService.IssueAPIKey(r.Context(), req.Tenant, req.Label)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusCreated, issued)
		case r.URL.Path == "/admin/api/keys/revoke" && r.Method == http.MethodPost:
			var req struct {
				Tenant string `json:"tenant"`
				KeyID  string `json:"key_id"`
			}
			if err := decodeAdminJSON(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			if err := cfg.AdminService.RevokeAPIKey(r.Context(), req.Tenant, req.KeyID); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
		case r.URL.Path == "/admin/api/identity-documents" && r.Method == http.MethodGet && cfg.IdentityDocuments != nil:
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			documents, err := cfg.IdentityDocuments.List(
				r.Context(),
				r.URL.Query().Get("tenant"),
				r.URL.Query().Get("person_id"),
				limit,
			)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"documents": documents})
		case r.URL.Path == "/admin/api/identity-documents" && r.Method == http.MethodPost && cfg.IdentityDocuments != nil:
			var document store.IdentityDocument
			if err := decodeAdminJSON(r, &document); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			saved, err := cfg.IdentityDocuments.Save(r.Context(), document, cfg.AdminUsername)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, saved)
		case r.URL.Path == "/admin/api/identity-documents/extract" && r.Method == http.MethodPost && cfg.DocumentExtractor != nil:
			r.Body = http.MaxBytesReader(w, r.Body, maxAdminDocumentUpload)
			if err := r.ParseMultipartForm(maxAdminDocumentUpload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document must be a PDF, JPEG, or PNG no larger than 10 MB"})
				return
			}
			defer func() {
				if r.MultipartForm != nil {
					_ = r.MultipartForm.RemoveAll()
				}
			}()
			file, header, err := r.FormFile("document")
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document file is required"})
				return
			}
			defer func() {
				_ = file.Close()
			}()

			prefix := make([]byte, 512)
			read, readErr := file.Read(prefix)
			if readErr != nil && readErr != io.EOF {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read document failed"})
				return
			}
			prefix = prefix[:read]
			contentType := http.DetectContentType(prefix)
			if !allowedDocumentUpload(header.Filename, contentType) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only PDF, JPEG, and PNG documents are supported"})
				return
			}

			extraction, err := cfg.DocumentExtractor.Extract(
				r.Context(),
				"", // admin console uses the server-default AI key
				filepath.Base(header.Filename),
				contentType,
				io.MultiReader(bytes.NewReader(prefix), file),
			)
			if err != nil {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, extraction)
		case r.URL.Path == "/admin/api/ai-status" && r.Method == http.MethodGet && cfg.AIService != nil:
			status, err := cfg.AIService.DefaultStatus(r.Context())
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ai status failed"})
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, status)
		case r.URL.Path == "/admin/api/ai-credentials" && r.Method == http.MethodPost && cfg.AIService != nil:
			var req struct {
				APIKey string `json:"api_key"`
				Model  string `json:"model"`
			}
			if err := decodeAdminJSON(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			if strings.TrimSpace(req.APIKey) == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "api_key is required"})
				return
			}
			meta, err := cfg.AIService.SetDefault(r.Context(), req.APIKey, req.Model)
			if err != nil {
				switch {
				case errors.Is(err, ai.ErrInvalidKey):
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "api key rejected by provider"})
				case errors.Is(err, ai.ErrStorageDisabled):
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "set AI_ENCRYPTION_KEY to store a key in the UI (the .env ANTHROPIC_API_KEY is already active)"})
				default:
					writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not validate api key"})
				}
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, meta)
		case r.URL.Path == "/admin/api/ai-credentials" && r.Method == http.MethodDelete && cfg.AIService != nil:
			if err := cfg.AIService.DeleteDefault(r.Context()); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "delete failed"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
		case r.URL.Path == "/admin/api/ai-validate" && r.Method == http.MethodPost && cfg.AIService != nil:
			model, err := cfg.AIService.ValidateDefault(r.Context())
			if err != nil {
				switch {
				case errors.Is(err, ai.ErrNotConfigured):
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no default key configured (set ANTHROPIC_API_KEY or pair one)"})
				case errors.Is(err, ai.ErrInvalidKey):
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "api key rejected by provider"})
				default:
					writeJSON(w, http.StatusBadGateway, map[string]string{"error": "could not reach the provider"})
				}
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "provider": cfg.AIService.ProviderName(), "model": model})
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		}
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/admin/api/login" && r.Method == http.MethodPost:
			var req struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := decodeAdminJSON(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
				return
			}
			if err := adminAuth.login(w, r, req.Username, req.Password); err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "authenticated"})
		case r.URL.Path == "/admin/api/logout" && r.Method == http.MethodPost:
			adminAuth.logout(w, r)
			writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
		default:
			protected.ServeHTTP(w, r)
		}
	})
}

func allowedDocumentUpload(filename, contentType string) bool {
	extension := strings.ToLower(filepath.Ext(filename))
	switch contentType {
	case "application/pdf":
		return extension == ".pdf"
	case "image/jpeg":
		return extension == ".jpg" || extension == ".jpeg"
	case "image/png":
		return extension == ".png"
	default:
		return false
	}
}

func decodeAdminJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeHTML(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(content))
}

const landingPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>KD-Server</title><style>
body{margin:0;font:16px system-ui;background:#08111f;color:#e8eef7}.wrap{max-width:900px;margin:0 auto;padding:72px 24px}
.tag{color:#65d6ad;font-weight:700;letter-spacing:.12em}.card{margin-top:28px;padding:28px;border:1px solid #29415e;border-radius:18px;background:#101d2d}
h1{font-size:54px;margin:8px 0 16px}p{color:#adbed1;line-height:1.65}.links{display:flex;gap:12px;flex-wrap:wrap;margin-top:24px}
a{color:#08111f;background:#65d6ad;padding:12px 18px;border-radius:10px;text-decoration:none;font-weight:700}a.alt{background:#1b3048;color:#e8eef7}
code{color:#9ee7cf}</style></head><body><main class="wrap"><div class="tag">MULTI-TENANT ENTITY API</div>
<h1>KD-Server</h1><p>A tenant-isolated Go and MongoDB service for ingesting, resolving, and looking up shared entity records.</p>
<section class="card"><strong>Service endpoints</strong><p><code>GET /v1/healthz</code> is public. Protected API routes require a bearer credential and <code>X-KD-Tenant</code>.</p>
<div class="links"><a href="/admin">Tenant administration</a><a class="alt" href="/v1/healthz">Health check</a></div></section>
</main></body></html>`

const identityDocumentPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>KD-Server · Identity documents</title><style>
*{box-sizing:border-box}body{margin:0;font:15px system-ui;background:#f3f6fa;color:#172235}.shell{max-width:1100px;margin:auto;padding:36px 22px}
header{display:flex;justify-content:space-between;align-items:center;margin-bottom:24px}h1{margin:0;font-size:30px}.muted{color:#607086}
.grid{display:grid;grid-template-columns:1fr 1fr;gap:18px}.card{background:white;border:1px solid #dce4ee;border-radius:14px;padding:20px;box-shadow:0 8px 24px #18324b0d}.wide{grid-column:1/-1}
label{display:block;font-weight:650;margin:12px 0 6px}input,select{width:100%;padding:11px;border:1px solid #bdcad8;border-radius:8px;background:white}
button{margin-top:14px;border:0;border-radius:8px;padding:11px 15px;background:#126b53;color:white;font-weight:700;cursor:pointer}.danger{background:#a83838}
#message,#secret{margin-top:16px;padding:12px;border-radius:8px;display:none;white-space:pre-wrap}.ok{display:block!important;background:#e5f6ef;color:#145b46}.err{display:block!important;background:#fdeaea;color:#8b2929}
table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:10px;border-bottom:1px solid #e7edf4}.hidden{display:none}.header-actions{display:flex;gap:10px;align-items:center}.header-actions button{margin:0}
.fields{display:grid;grid-template-columns:repeat(3,1fr);gap:0 14px}.check-row{display:flex;gap:18px;margin-top:14px}.check-row label{display:flex;align-items:center;gap:7px;margin:0}.check-row input{width:auto}.extract-box{margin:14px 0;padding:14px;border:1px solid #b9c9d8;border-radius:10px;background:#f7fafc}.extract-box input{background:white}.extract-status{margin-top:9px;font-size:12px;color:#42566b}.ocr-review{margin-top:12px}.ocr-review textarea{width:100%;min-height:120px;padding:10px;border:1px solid #bdcad8;border-radius:8px;font:12px ui-monospace,Consolas,monospace}.confidence{display:flex;gap:7px;flex-wrap:wrap;margin-top:8px}.confidence span{padding:4px 7px;border-radius:999px;background:#e8eef5;font-size:11px}
.identity-layout{display:grid;grid-template-columns:minmax(320px,1fr) minmax(360px,1.1fr);gap:22px}.document-list{display:flex;gap:8px;flex-wrap:wrap;margin:12px 0}.document-list button{margin:0;background:#e8eef5;color:#223247}
.id-preview{display:grid;gap:14px;justify-items:center;overflow:auto;padding:8px;background:#eef2f6;border-radius:12px}.document-warning{width:85.60mm;max-width:100%;font-size:11px;font-weight:750;text-align:center;color:#8b2929;letter-spacing:.06em}
.identity-card{width:85.60mm;height:53.98mm;border:1px solid #777;border-radius:4px;padding:3mm;font:10px Arial,Helvetica,sans-serif;color:#000;background:linear-gradient(145deg,#f9faf9,#edf1f0);position:relative;flex:none;overflow:hidden;isolation:isolate}
.identity-card:before{content:"";position:absolute;z-index:-1;left:18mm;bottom:7mm;width:28mm;height:20mm;opacity:.16;background:linear-gradient(135deg,transparent 38%,#d34a4a 39% 48%,transparent 49%),linear-gradient(45deg,transparent 38%,#3c72a8 39% 48%,transparent 49%),radial-gradient(circle at 55% 58%,#d6b529 0 25%,transparent 26%)}
.identity-card:after{content:"SAMPLE";position:absolute;z-index:4;left:50%;top:56%;transform:translate(-50%,-50%) rotate(-18deg);font-size:24px;font-weight:900;letter-spacing:.24em;color:#9d2d2d1c;pointer-events:none}
.identity-card .card-header{height:15mm;width:100%;text-align:center}.identity-card .card-title{font-size:12px;font-weight:bold}.identity-card .card-type,.identity-card .id-number,.identity-card .flex{display:flex;justify-content:space-between}
.identity-card .id-number{height:5mm}.identity-card .card-number,.identity-card .blood-text,.identity-card .label{font-weight:bold}.identity-card .info{display:flex;width:100%;height:35mm}.identity-card .info-data{flex:1;min-width:0}.identity-card .name{font-size:xx-small}
.dv{font-family:"Faruma","MV Boli","Noto Sans Thaana",sans-serif;direction:rtl;text-align:right}.identity-card .sex-dob-row,.identity-card .blood-expiry,.identity-card .back-data{display:flex}.identity-card .sex{width:30mm}.identity-card .dob{width:100%}
.identity-card .p2{padding:2px}.identity-card .border{border:1px solid #000}.identity-card .border-left{border-left:1px solid #000}.identity-card .border-right{border-right:1px solid #000}.identity-card .border-top{border-top:1px solid #000}.identity-card .border-bottom{border-bottom:1px solid #000}.identity-card .center{text-align:center}
.identity-card .photo{width:25mm;height:35mm;background:#dfe4e3;display:grid;place-items:end center;color:#666;font-size:8px;overflow:hidden}.identity-card .photo svg{width:22mm;height:32mm;display:block}.identity-card .serial{height:4mm}.identity-card .magnetic-stripe{height:10mm;width:100%;background:#202020}.identity-card .signature{width:32mm}.identity-card .signature-image{width:30mm;height:30mm;background:#e5e5e5;display:grid;place-items:center;color:#666;font-size:8px}.identity-card .back-info{width:100%}.identity-card .blank-row{height:8mm}.identity-card .blood{width:25mm}.identity-card .expiry{flex-grow:1}.identity-card .trace-note{position:absolute;z-index:5;right:3mm;top:1.5mm;font-size:7px;color:#8b2929;font-weight:bold}
@media(max-width:900px){.identity-layout{grid-template-columns:1fr}.fields{grid-template-columns:1fr 1fr}}@media(max-width:760px){.grid{grid-template-columns:1fr}.fields{grid-template-columns:1fr}.wide{grid-column:auto}}
</style></head><body><main class="shell"><header><div><h1>Identity documents</h1><div class="muted">Testing tool — extract and record Maldivian identity documents.</div></div><div class="header-actions"><a href="/admin">&larr; Admin console</a><button id="logoutButton" class="danger hidden" onclick="logout()">Log out</button></div></header>
<div id="message"></div>
<section id="loginCard" class="card"><h2>Administrator login</h2><label for="username">User ID</label><input id="username" autocomplete="username" placeholder="Admin user ID"><label for="password">Password</label><input id="password" type="password" autocomplete="current-password"><button onclick="login()">Sign in</button></section>
<div id="adminDashboard" class="hidden" style="margin-top:18px">
<section class="card wide"><h2>Maldivian identity document</h2><p class="muted">Structured document tracing linked to a person. Image fields store secured references only.</p>
<div class="identity-layout"><div>
<div class="extract-box"><strong>Extract from document copy</strong><div class="muted">PDF, JPG, or PNG up to 10 MB. Processing is local and temporary; review results before saving.</div><input id="identityDocumentFile" type="file" accept=".pdf,.jpg,.jpeg,.png,application/pdf,image/jpeg,image/png"><button type="button" onclick="extractIdentityDocument()">Extract fields</button><div id="extractStatus" class="extract-status"></div><div id="ocrReview" class="ocr-review hidden"><label>Raw OCR text</label><textarea id="rawOCRText" readonly></textarea><div id="ocrConfidence" class="confidence"></div></div></div>
<input id="documentId" type="hidden">
<div class="fields">
<div><label>Tenant</label><select id="documentTenant" onchange="loadIdentityDocuments()"></select></div>
<div><label>Person ID</label><input id="personId" placeholder="Resolved KD person ID"></div>
<div><label>National ID</label><input id="nationalId" placeholder="A000000"></div>
<div><label>English name</label><input id="nameEnglish" placeholder="Sample Person"></div>
<div><label>Dhivehi name</label><input id="nameDhivehi" class="dv" placeholder="ނަމޫނާ މީހެއް"></div>
<div><label>Common name</label><input id="commonNameEnglish" placeholder="Sample"></div>
<div><label>Common name (Dhivehi)</label><input id="commonNameDhivehi" class="dv" placeholder="ނަމޫނާ"></div>
<div><label>Sex</label><select id="identitySex"><option value="">Not recorded</option><option value="M">M</option><option value="F">F</option></select></div>
<div><label>Date of birth</label><input id="dateOfBirth" type="date"></div>
<div><label>House</label><input id="houseEnglish" placeholder="Example House"></div>
<div><label>House (Dhivehi)</label><input id="houseDhivehi" class="dv" placeholder="ނަމޫނާގެ"></div>
<div><label>Island</label><input id="islandEnglish" placeholder="K. Male"></div>
<div><label>Island (Dhivehi)</label><input id="islandDhivehi" class="dv" placeholder="ކ. މާލެ"></div>
<div><label>Blood group</label><input id="bloodGroup" placeholder="A+"></div>
<div><label>Expiry date</label><input id="expiryDate" type="date"></div>
<div><label>Serial number</label><input id="serialNumber" placeholder="Document serial"></div>
<div><label>Source</label><input id="identitySource" value="manual-admin"></div>
<div><label>Extraction</label><select id="extractionMethod"><option value="manual">Manual</option><option value="ocr">OCR</option><option value="source_import">Source import</option></select></div>
<div><label>Verification</label><select id="verificationStatus"><option value="unverified">Unverified</option><option value="verified">Verified</option><option value="rejected">Rejected</option></select></div>
<div><label>Verified by</label><input id="verifiedBy" placeholder="Staff user ID"></div>
<div><label>Front image reference</label><input id="frontImageRef" placeholder="secure://..."></div>
<div><label>Back image reference</label><input id="backImageRef" placeholder="secure://..."></div>
</div>
<div class="check-row"><label><input id="signaturePresent" type="checkbox"> Signature present</label><label><input id="fingerprintPresent" type="checkbox"> Fingerprint present</label></div>
<button onclick="saveIdentityDocument()">Save document</button><button type="button" style="background:#50657a;margin-left:8px" onclick="newIdentityDocument()">New</button>
<div id="identityDocuments" class="document-list"></div></div>
<div class="id-preview"><div class="document-warning">DATA PREVIEW - NOT A VALID IDENTITY CREDENTIAL</div>
<div class="identity-card">
<div class="card-header"><div class="card-title dv">ދިވެހި ޖުމްހޫރިއްޔާ</div><div class="card-title">REPUBLIC OF MALDIVES</div><div class="card-type"><div>NATIONAL IDENTITY CARD</div><div class="dv">ދިވެހި ރައްޔިތެއްކަން އަންގައިދޭ ކާޑު</div></div></div>
<div class="id-number p2"><div class="label">Number:</div><div id="previewNationalId" class="card-number">A000000</div><div class="label dv">ނަންބަރު:</div></div>
<div class="info"><div class="info-data border">
<div class="name border-bottom p2"><div class="flex"><div class="label">Name</div><div class="label dv">ނަން</div></div><div id="previewNameDhivehi" class="dv">ނަމޫނާ މީހެއް</div><div id="previewNameEnglish">Sample Person</div></div>
<div class="sex-dob-row border-bottom"><div class="sex p2"><div class="flex"><div class="label">Sex</div><div class="label dv">ޖިންސު</div></div><div class="flex"><div id="previewSexEnglish">-</div><div id="previewSexDhivehi" class="dv">-</div></div></div>
<div class="dob p2"><div class="flex"><div class="label">Date of Birth</div><div class="label dv">އުފަން ތާރީޚް</div></div><div id="previewDateOfBirth" class="center">-</div></div></div>
<div class="p2"><div class="flex"><div class="label">Address</div><div class="label dv">އެޑްރެސް</div></div><div class="flex"><div id="previewHouseEnglish">Example House</div><div id="previewHouseDhivehi" class="dv">ނަމޫނާގެ</div></div><div class="flex"><div id="previewIslandEnglish">K. Male</div><div id="previewIslandDhivehi" class="dv">ކ. މާލެ</div></div></div>
</div><div class="photo p2" aria-label="Fictional sample portrait"><svg viewBox="0 0 100 130" role="img" aria-label="Generic person silhouette"><rect width="100" height="130" fill="#dfe4e3"/><circle cx="50" cy="38" r="24" fill="#778582"/><path d="M16 130c2-37 15-56 34-56s32 19 34 56" fill="#667572"/><path d="M29 37c1-23 12-34 22-34 15 0 24 14 22 35-7-8-13-11-23-11-8 0-14 3-21 10z" fill="#394542"/></svg></div></div></div>
<div class="identity-card"><div id="previewTrace" class="trace-note">UNSAVED / UNVERIFIED</div><div id="previewSerial" class="serial">-</div><div class="magnetic-stripe"></div>
<div class="back-data"><div class="signature p2"><div class="dv">ސޮއި / އިނގިލީގެ ނިޝާން</div><div>Signature / Finger Print</div><div id="previewSignature" class="signature-image">NOT RECORDED</div></div>
<div class="back-info"><div class="blank-row"></div><div class="border p2"><div class="flex"><div class="label">Common Name</div><div class="label dv">ޢާއްމު ނަން</div></div><div class="flex"><div id="previewCommonNameEnglish" class="label">Sample</div><div id="previewCommonNameDhivehi" class="label dv">ނަމޫނާ</div></div></div>
<div class="blank-row"></div><div class="blood-expiry border-top border-bottom"><div class="blood border-left border-right p2"><div class="flex"><div class="label">Blood Group</div><div class="label dv">ލޭގެ ގްރޫޕް</div></div><div id="previewBloodGroup" class="blood-text center">-</div></div>
<div class="expiry border-right p2"><div class="flex"><div class="label">Expiry Date</div><div class="label dv">މުއްދަތު ހަމަވަނީ</div></div><div id="previewExpiry" class="center">-</div></div></div></div></div></div>
</div></div></section></div>
</main><script>
function show(text,error=false){const el=document.getElementById('message');el.textContent=text;el.className=error?'err':'ok'}
async function request(path,options={}){const res=await fetch(path,{...options,headers:{'Content-Type':'application/json',...(options.headers||{})}});const data=await res.json();if(!res.ok)throw new Error(data.error||'Request failed');return data}
async function login(){try{await request('/admin/api/login',{method:'POST',body:JSON.stringify({username:value('username'),password:value('password')})});document.getElementById('password').value='';await loadTenants();await loadIdentityDocuments();setAuthenticated(true);show('Signed in.')}catch(e){show(e.message,true)}}
async function logout(){await request('/admin/api/logout',{method:'POST'});setAuthenticated(false);show('Signed out.')}
function setAuthenticated(authenticated){document.getElementById('loginCard').classList.toggle('hidden',authenticated);document.getElementById('adminDashboard').classList.toggle('hidden',!authenticated);document.getElementById('logoutButton').classList.toggle('hidden',!authenticated)}
let tenants=[];let identityDocuments=[];async function loadTenants(){const data=await request('/admin/api/tenants');tenants=data.tenants;const s=document.getElementById('documentTenant');s.innerHTML=tenants.map(t=>'<option value="'+escapeHTML(t.slug)+'">'+escapeHTML(t.name)+' ('+escapeHTML(t.slug)+')</option>').join('')}
function identityPayload(){return{document_id:value('documentId'),tenant_id:value('documentTenant'),person_id:value('personId'),national_id:value('nationalId'),serial_number:value('serialNumber'),name:{english:value('nameEnglish'),dhivehi:value('nameDhivehi')},common_name:{english:value('commonNameEnglish'),dhivehi:value('commonNameDhivehi')},sex:value('identitySex'),date_of_birth:value('dateOfBirth'),address:{house:{english:value('houseEnglish'),dhivehi:value('houseDhivehi')},island:{english:value('islandEnglish'),dhivehi:value('islandDhivehi')}},blood_group:value('bloodGroup'),expiry_date:value('expiryDate'),signature_present:checked('signaturePresent'),fingerprint_present:checked('fingerprintPresent'),source:value('identitySource'),extraction_method:value('extractionMethod'),verification_status:value('verificationStatus'),verified_by:value('verifiedBy'),front_image_ref:value('frontImageRef'),back_image_ref:value('backImageRef')}}
async function extractIdentityDocument(){const input=document.getElementById('identityDocumentFile');if(!input.files.length){show('Choose a PDF, JPG, or PNG document first.',true);return}const status=document.getElementById('extractStatus');status.textContent='Extracting locally...';const body=new FormData();body.append('document',input.files[0]);try{const res=await fetch('/admin/api/identity-documents/extract',{method:'POST',body});const data=await res.json();if(!res.ok)throw new Error(data.error||'Extraction failed');applyExtraction(data);status.textContent='Processed '+data.pages_processed+' page(s) with '+data.engine+'. Review every field before saving.';show((data.warnings||[]).join('\n')||'Document fields extracted for review.')}catch(e){status.textContent='';show(e.message,true)}}
function applyExtraction(data){setIfPresent('nationalId',data.national_id);setIfPresent('nameEnglish',data.name_english);setIfPresent('nameDhivehi',data.name_dhivehi);setIfPresent('commonNameEnglish',data.common_name_english);setIfPresent('identitySex',data.sex);setIfPresent('dateOfBirth',data.date_of_birth);setIfPresent('houseEnglish',data.house_english);setIfPresent('houseDhivehi',data.house_dhivehi);setIfPresent('islandEnglish',data.island_english);setIfPresent('islandDhivehi',data.island_dhivehi);setIfPresent('bloodGroup',data.blood_group);setIfPresent('expiryDate',data.expiry_date);setIfPresent('serialNumber',data.serial_number);setValue('identitySource','document-upload');setValue('extractionMethod','ocr');setValue('verificationStatus','unverified');document.getElementById('rawOCRText').value=data.raw_text||'';const confidence=document.getElementById('ocrConfidence');confidence.innerHTML='';for(const [field,score] of Object.entries(data.field_confidence||{}))confidence.insertAdjacentHTML('beforeend','<span>'+escapeHTML(field)+': '+Math.round(score*100)+'%</span>');document.getElementById('ocrReview').classList.remove('hidden');updateIdentityPreview()}
async function saveIdentityDocument(){try{const saved=await request('/admin/api/identity-documents',{method:'POST',body:JSON.stringify(identityPayload())});document.getElementById('documentId').value=saved.document_id;await loadIdentityDocuments();show('Identity document saved with trace version '+saved.version+'.')}catch(e){show(e.message,true)}}
async function loadIdentityDocuments(){const tenant=value('documentTenant');if(!tenant)return;try{const data=await request('/admin/api/identity-documents?tenant='+encodeURIComponent(tenant));identityDocuments=data.documents||[];const list=document.getElementById('identityDocuments');list.innerHTML=identityDocuments.length?'':'<span class="muted">No saved identity documents for this tenant.</span>';identityDocuments.forEach((d,i)=>list.insertAdjacentHTML('beforeend','<button type="button" onclick="selectIdentityDocument('+i+')">'+escapeHTML(d.national_id)+' • '+escapeHTML((d.name||{}).english||'Unnamed')+'</button>'))}catch(e){show(e.message,true)}}
function selectIdentityDocument(index){const d=identityDocuments[index];if(!d)return;setValue('documentId',d.document_id);setValue('personId',d.person_id);setValue('nationalId',d.national_id);setValue('serialNumber',d.serial_number);setValue('nameEnglish',d.name&&d.name.english);setValue('nameDhivehi',d.name&&d.name.dhivehi);setValue('commonNameEnglish',d.common_name&&d.common_name.english);setValue('commonNameDhivehi',d.common_name&&d.common_name.dhivehi);setValue('identitySex',d.sex);setValue('dateOfBirth',d.date_of_birth);setValue('houseEnglish',d.address&&d.address.house&&d.address.house.english);setValue('houseDhivehi',d.address&&d.address.house&&d.address.house.dhivehi);setValue('islandEnglish',d.address&&d.address.island&&d.address.island.english);setValue('islandDhivehi',d.address&&d.address.island&&d.address.island.dhivehi);setValue('bloodGroup',d.blood_group);setValue('expiryDate',d.expiry_date);setValue('identitySource',d.source);setValue('extractionMethod',d.extraction_method);setValue('verificationStatus',d.verification_status);setValue('verifiedBy',d.verified_by);setValue('frontImageRef',d.front_image_ref);setValue('backImageRef',d.back_image_ref);document.getElementById('signaturePresent').checked=!!d.signature_present;document.getElementById('fingerprintPresent').checked=!!d.fingerprint_present;updateIdentityPreview()}
function newIdentityDocument(){for(const id of ['documentId','personId','nationalId','serialNumber','nameEnglish','nameDhivehi','commonNameEnglish','commonNameDhivehi','dateOfBirth','houseEnglish','houseDhivehi','islandEnglish','islandDhivehi','bloodGroup','expiryDate','verifiedBy','frontImageRef','backImageRef'])setValue(id,'');setValue('identitySource','manual-admin');setValue('extractionMethod','manual');setValue('verificationStatus','unverified');setValue('identitySex','');document.getElementById('signaturePresent').checked=false;document.getElementById('fingerprintPresent').checked=false;updateIdentityPreview()}
function updateIdentityPreview(){const sex=value('identitySex');text('previewNationalId',value('nationalId')||'A000000');text('previewNameEnglish',value('nameEnglish')||'Sample Person');text('previewNameDhivehi',value('nameDhivehi')||'ނަމޫނާ މީހެއް');text('previewHouseEnglish',value('houseEnglish')||'Example House');text('previewHouseDhivehi',value('houseDhivehi')||'ނަމޫނާގެ');text('previewIslandEnglish',value('islandEnglish')||'K. Male');text('previewIslandDhivehi',value('islandDhivehi')||'ކ. މާލެ');text('previewSexEnglish',sex||'-');text('previewSexDhivehi',sex==='M'?'މ':sex==='F'?'އ':'-');text('previewDateOfBirth',displayDate(value('dateOfBirth')));text('previewExpiry',displayDate(value('expiryDate')));text('previewCommonNameEnglish',value('commonNameEnglish')||'Sample');text('previewCommonNameDhivehi',value('commonNameDhivehi')||'ނަމޫނާ');text('previewBloodGroup',value('bloodGroup')||'-');text('previewSerial',value('serialNumber')||'-');text('previewSignature',checked('signaturePresent')||checked('fingerprintPresent')?'REFERENCE RECORDED':'NOT RECORDED');text('previewTrace',(value('documentId')||'UNSAVED')+' / '+(value('verificationStatus')||'UNVERIFIED').toUpperCase())}
function displayDate(v){if(!v)return '-';const p=v.split('-');return p.length===3?p[2]+'/'+p[1]+'/'+p[0]:v}
for(const el of document.querySelectorAll('#adminDashboard input,#adminDashboard select'))el.addEventListener('input',updateIdentityPreview)
function checked(id){return document.getElementById(id).checked}function setValue(id,v){document.getElementById(id).value=v||''}function setIfPresent(id,v){if(v)setValue(id,v)}function text(id,v){document.getElementById(id).textContent=v}
function value(id){return document.getElementById(id).value.trim()}function escapeHTML(v){return String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
loadTenants().then(()=>loadIdentityDocuments()).then(()=>setAuthenticated(true)).catch(()=>setAuthenticated(false));updateIdentityPreview();
</script></body></html>`
