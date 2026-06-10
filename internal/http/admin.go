package http

import (
	"encoding/json"
	"net/http"
)

func landingHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeHTML(w, landingPage)
}

func adminPageHandler(w http.ResponseWriter, _ *http.Request) {
	writeHTML(w, adminPage)
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

const adminPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>KD-Server Admin</title><style>
*{box-sizing:border-box}body{margin:0;font:15px system-ui;background:#f3f6fa;color:#172235}.shell{max-width:1100px;margin:auto;padding:36px 22px}
header{display:flex;justify-content:space-between;align-items:center;margin-bottom:24px}h1{margin:0;font-size:30px}.muted{color:#607086}
.grid{display:grid;grid-template-columns:1fr 1fr;gap:18px}.card{background:white;border:1px solid #dce4ee;border-radius:14px;padding:20px;box-shadow:0 8px 24px #18324b0d}
label{display:block;font-weight:650;margin:12px 0 6px}input,select{width:100%;padding:11px;border:1px solid #bdcad8;border-radius:8px;background:white}
button{margin-top:14px;border:0;border-radius:8px;padding:11px 15px;background:#126b53;color:white;font-weight:700;cursor:pointer}.danger{background:#a83838}
#message,#secret{margin-top:16px;padding:12px;border-radius:8px;display:none;white-space:pre-wrap}.ok{display:block!important;background:#e5f6ef;color:#145b46}.err{display:block!important;background:#fdeaea;color:#8b2929}
table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:10px;border-bottom:1px solid #e7edf4}.hidden{display:none}.header-actions{display:flex;gap:10px;align-items:center}.header-actions button{margin:0}@media(max-width:760px){.grid{grid-template-columns:1fr}}
</style></head><body><main class="shell"><header><div><h1>Tenant administration</h1><div class="muted">Create tenants and manage API credentials.</div></div><div class="header-actions"><a href="/">KD-Server</a><button id="logoutButton" class="danger hidden" onclick="logout()">Log out</button></div></header>
<section id="loginCard" class="card"><h2>Administrator login</h2><label for="username">User ID</label><input id="username" autocomplete="username" placeholder="Admin user ID"><label for="password">Password</label><input id="password" type="password" autocomplete="current-password"><button onclick="login()">Sign in</button><div id="message"></div></section>
<div id="adminDashboard" class="grid hidden" style="margin-top:18px"><section class="card"><h2>Create tenant</h2><label>Name</label><input id="tenantName" placeholder="Family Court"><label>Slug</label><input id="tenantSlug" placeholder="fc"><button onclick="createTenant()">Create tenant</button></section>
<section class="card"><h2>Edit tenant</h2><label>Select tenant</label><select id="editTenant" onchange="selectTenantForEdit()"></select><div class="muted">The slug shown in parentheses is permanent because it identifies tenant data and credentials.</div><label>Display name</label><input id="editTenantName" placeholder="Updated tenant name"><button onclick="updateTenant()">Save changes</button></section>
<section class="card"><h2>Issue API key</h2><label>Tenant</label><select id="issueTenant"></select><label>Label</label><input id="keyLabel" placeholder="FC production"><button onclick="issueKey()">Issue key</button><div id="secret"></div></section>
<section class="card"><h2>Revoke API key</h2><label>Tenant</label><select id="revokeTenant"></select><label>Key ID</label><input id="keyId" placeholder="key_..."><button class="danger" onclick="revokeKey()">Revoke key</button></section>
<section class="card"><h2>Tenants</h2><table><thead><tr><th>Slug</th><th>Name</th><th>Created</th></tr></thead><tbody id="tenants"></tbody></table></section></div>
</main><script>
function show(text,error=false){const el=document.getElementById('message');el.textContent=text;el.className=error?'err':'ok'}
async function request(path,options={}){const res=await fetch(path,{...options,headers:{'Content-Type':'application/json',...(options.headers||{})}});const data=await res.json();if(!res.ok)throw new Error(data.error||'Request failed');return data}
async function login(){try{await request('/admin/api/login',{method:'POST',body:JSON.stringify({username:value('username'),password:value('password')})});document.getElementById('password').value='';await loadTenants();setAuthenticated(true);show('Signed in.')}catch(e){show(e.message,true)}}
async function logout(){await request('/admin/api/logout',{method:'POST'});setAuthenticated(false);show('Signed out.')}
function setAuthenticated(authenticated){document.getElementById('loginCard').classList.toggle('hidden',authenticated);document.getElementById('adminDashboard').classList.toggle('hidden',!authenticated);document.getElementById('logoutButton').classList.toggle('hidden',!authenticated)}
let tenants=[];async function loadTenants(){const data=await request('/admin/api/tenants');tenants=data.tenants;const rows=document.getElementById('tenants');rows.innerHTML='';for(const t of tenants){rows.insertAdjacentHTML('beforeend','<tr><td>'+escapeHTML(t.slug)+'</td><td>'+escapeHTML(t.name)+'</td><td>'+new Date(t.created_at).toLocaleDateString()+'</td></tr>')}for(const id of ['editTenant','issueTenant','revokeTenant']){const s=document.getElementById(id);s.innerHTML=tenants.map(t=>'<option value="'+escapeHTML(t.slug)+'">'+escapeHTML(t.name)+' ('+escapeHTML(t.slug)+')</option>').join('')}selectTenantForEdit()}
async function createTenant(){try{await request('/admin/api/tenants',{method:'POST',body:JSON.stringify({name:value('tenantName'),slug:value('tenantSlug')})});await loadTenants();show('Tenant created.')}catch(e){show(e.message,true)}}
function selectTenantForEdit(){const tenant=tenants.find(t=>t.slug===value('editTenant'));document.getElementById('editTenantName').value=tenant?tenant.name:''}
async function updateTenant(){try{await request('/admin/api/tenants/update',{method:'POST',body:JSON.stringify({slug:value('editTenant'),name:value('editTenantName')})});await loadTenants();show('Tenant details updated. Existing API keys remain valid.')}catch(e){show(e.message,true)}}
async function issueKey(){try{const data=await request('/admin/api/keys',{method:'POST',body:JSON.stringify({tenant:value('issueTenant'),label:value('keyLabel')})});const el=document.getElementById('secret');el.className='ok';el.textContent='Save this secret now. It will not be shown again:\n\n'+data.secret+'\n\nKey ID: '+data.key_id}catch(e){show(e.message,true)}}
async function revokeKey(){if(!confirm('Revoke this API key? Existing clients using it will stop working.'))return;try{await request('/admin/api/keys/revoke',{method:'POST',body:JSON.stringify({tenant:value('revokeTenant'),key_id:value('keyId')})});show('API key revoked.')}catch(e){show(e.message,true)}}
function value(id){return document.getElementById(id).value.trim()}function escapeHTML(v){return String(v).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
loadTenants().then(()=>setAuthenticated(true)).catch(()=>setAuthenticated(false));
</script></body></html>`
