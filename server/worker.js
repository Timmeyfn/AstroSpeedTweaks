// AstroSpeed key server — runs free on Cloudflare Workers.
//
// What it does:
//   • /activate  {key, hwid}  → ties a key to the FIRST PC that uses it; refuses it on any other PC.
//   • /check     {key, hwid}  → the app re-checks on launch; a disabled key locks the app.
//   • /admin                  → a password-protected page where YOU make keys and disable them.
//
// Setup is in "SETUP THE KEY SERVER.txt". You need:
//   • a KV namespace bound as  LICENSES
//   • a variable  ADMIN_PASS  set to a password only you know

const CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "POST, GET, OPTIONS",
  "Access-Control-Allow-Headers": "Content-Type",
};

export default {
  async fetch(req, env) {
    const url = new URL(req.url);
    if (req.method === "OPTIONS") return new Response(null, { headers: CORS });
    try {
      if (url.pathname === "/activate") return await activate(req, env);
      if (url.pathname === "/check") return await check(req, env);
      if (url.pathname === "/admin") return await admin(req, env);
    } catch (e) {
      return json({ ok: false, reason: "Server error" }, 500);
    }
    return new Response("AstroSpeed key server is running.", { headers: CORS });
  },
};

function json(obj, status = 200) {
  return new Response(JSON.stringify(obj), { status, headers: { "Content-Type": "application/json", ...CORS } });
}
function normKey(k) { return String(k || "").toUpperCase().replace(/[^A-Z0-9]/g, "").replace(/(.{5})/g, "$1-").replace(/-$/, ""); }

async function activate(req, env) {
  const { key, hwid } = await req.json().catch(() => ({}));
  const k = normKey(key);
  if (!k || !hwid) return json({ ok: false, reason: "Enter your key." });
  const rec = await env.LICENSES.get(k, "json");
  if (!rec) return json({ ok: false, reason: "That key isn't valid." });
  if (rec.revoked) return json({ ok: false, reason: "This key has been disabled." });
  if (rec.hwid && rec.hwid !== hwid) return json({ ok: false, reason: "This key is already in use on another PC." });
  if (!rec.hwid) { rec.hwid = hwid; rec.activatedAt = Date.now(); await env.LICENSES.put(k, JSON.stringify(rec)); }
  return json({ ok: true });
}

async function check(req, env) {
  const { key, hwid } = await req.json().catch(() => ({}));
  const k = normKey(key);
  const rec = await env.LICENSES.get(k, "json");
  const ok = !!rec && !rec.revoked && rec.hwid === hwid;
  return json({ ok, reason: ok ? "" : (rec && rec.revoked ? "disabled" : "invalid") });
}

// ───────────────────────── admin ─────────────────────────

async function admin(req, env) {
  if (req.method === "GET") return new Response(ADMIN_HTML, { headers: { "Content-Type": "text/html; charset=utf-8" } });
  const body = await req.json().catch(() => ({}));
  if (!env.ADMIN_PASS || body.pass !== env.ADMIN_PASS) return json({ ok: false, reason: "Wrong password" }, 401);
  const action = body.action;
  if (action === "list") {
    const list = await env.LICENSES.list({ limit: 1000 });
    const rows = [];
    for (const it of list.keys) {
      const rec = await env.LICENSES.get(it.name, "json");
      if (rec) rows.push({ key: it.name, ...rec, hwid: rec.hwid ? rec.hwid.slice(0, 10) + "…" : "" });
    }
    rows.sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));
    return json({ ok: true, rows });
  }
  if (action === "generate") {
    const n = Math.min(Math.max(parseInt(body.count) || 1, 1), 50);
    const made = [];
    for (let i = 0; i < n; i++) {
      const key = newKey();
      await env.LICENSES.put(key, JSON.stringify({ createdAt: Date.now(), hwid: null, revoked: false, note: body.note || "" }));
      made.push(key);
    }
    return json({ ok: true, made });
  }
  if (action === "revoke" || action === "enable") {
    const rec = await env.LICENSES.get(body.key, "json");
    if (!rec) return json({ ok: false, reason: "No such key" });
    rec.revoked = action === "revoke";
    await env.LICENSES.put(body.key, JSON.stringify(rec));
    return json({ ok: true });
  }
  if (action === "unbind") {
    const rec = await env.LICENSES.get(body.key, "json");
    if (!rec) return json({ ok: false, reason: "No such key" });
    rec.hwid = null; rec.activatedAt = null;
    await env.LICENSES.put(body.key, JSON.stringify(rec));
    return json({ ok: true });
  }
  if (action === "delete") { await env.LICENSES.delete(body.key); return json({ ok: true }); }
  return json({ ok: false, reason: "Unknown action" });
}

function newKey() {
  const abc = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"; // no 0/O/1/I/L
  let out = "ASTRO";
  const a = new Uint8Array(15);
  crypto.getRandomValues(a);
  for (let i = 0; i < 15; i++) { if (i % 5 === 0) out += "-"; out += abc[a[i] % abc.length]; }
  return out;
}

const ADMIN_HTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>AstroSpeed keys</title>
<style>
*{box-sizing:border-box;font-family:system-ui,"Segoe UI",sans-serif}body{margin:0;background:#0F0F10;color:#F1EFEA;padding:24px}
h1{font-size:20px;margin:0 0 16px}input,button,select{font:inherit}
.bar{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-bottom:18px}
input{background:#18181A;border:1px solid #333;border-radius:8px;color:#fff;padding:9px 12px}
button{background:#E2542A;border:0;border-radius:8px;color:#fff;font-weight:700;padding:9px 14px;cursor:pointer}
button.g{background:#2a2a2e}
table{width:100%;border-collapse:collapse;font-size:13px}th,td{text-align:left;padding:8px 10px;border-bottom:1px solid #2a2a2e}
.k{font-family:ui-monospace,Consolas,monospace;font-weight:700}
.tag{font-size:11px;font-weight:700;padding:2px 8px;border-radius:999px}
.unused{background:#30465a;color:#9cd}.active{background:#1f5137;color:#8f8}.revoked{background:#5a2330;color:#f99}
.made{background:#18181A;border:1px solid #333;border-radius:8px;padding:12px;margin-bottom:14px;white-space:pre-wrap;font-family:ui-monospace,monospace;font-size:13px}
</style></head><body>
<h1>AstroSpeed keys</h1>
<div class="bar"><input id="pass" type="password" placeholder="Admin password"><button onclick="load()">Sign in</button></div>
<div id="made"></div>
<div class="bar" id="tools" style="display:none"><input id="count" type="number" value="1" min="1" max="50" style="width:70px"><input id="note" placeholder="note (optional, e.g. buyer email)"><button onclick="gen()">Generate key(s)</button><button class="g" onclick="load()">Refresh</button></div>
<table id="tbl"></table>
<script>
const P=()=>document.getElementById('pass').value;
async function call(action,extra){const r=await fetch('/admin',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({pass:P(),action,...extra})});return r.json();}
async function load(){const d=await call('list',{});if(!d.ok){alert(d.reason||'Error');return;}document.getElementById('tools').style.display='flex';const rows=d.rows||[];document.getElementById('tbl').innerHTML='<tr><th>Key</th><th>Status</th><th>PC</th><th>Note</th><th></th></tr>'+rows.map(r=>{const st=r.revoked?'revoked':(r.hwid?'active':'unused');return '<tr><td class=k>'+r.key+'</td><td><span class="tag '+st+'">'+st+'</span></td><td>'+(r.hwid||'')+'</td><td>'+(r.note||'')+'</td><td>'+(r.revoked?'<button class=g onclick="act(\\''+r.key+'\\',\\'enable\\')">Enable</button>':'<button onclick="act(\\''+r.key+'\\',\\'revoke\\')">Disable</button>')+' <button class=g onclick="act(\\''+r.key+'\\',\\'unbind\\')">Unbind PC</button></td></tr>';}).join('');}
async function gen(){const d=await call('generate',{count:document.getElementById('count').value,note:document.getElementById('note').value});if(!d.ok){alert(d.reason);return;}document.getElementById('made').innerHTML='<div class=made>New key(s) — copy and send to the buyer:\\n\\n'+d.made.join('\\n')+'</div>';load();}
async function act(key,a){if(a==='revoke'&&!confirm('Disable '+key+'? The app will lock on that PC next launch.'))return;await call(a,{key});load();}
</script></body></html>`;
