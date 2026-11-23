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
      if (rec) rows.push({ key: it.name, ...rec });
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

const ADMIN_HTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>AstroSpeed keys</title>
<style>
*{box-sizing:border-box;margin:0;padding:0;font-family:"Manrope",system-ui,"Segoe UI",sans-serif}
:root{--bg:#0B0B0C;--card:#17171A;--card2:#1E1E22;--ink:#F3F1EC;--ink2:#A6A8AE;--ink3:#6B6E74;--line:rgba(255,255,255,.1);--accent:#FF6B3F;--ok:#4FC27E;--warn:#F2B347;--bad:#FF5A6E}
body{background:var(--bg);color:var(--ink);padding:26px;max-width:1120px;margin:0 auto}
.hd{display:flex;align-items:center;gap:12px;margin-bottom:22px}
.mk{width:38px;height:38px;border-radius:11px;background:linear-gradient(140deg,#FF3D6E,#FF6B3F);display:grid;place-items:center}
.mk svg{width:21px;height:21px}
h1{font-size:21px;font-weight:800;letter-spacing:-.02em}
.sub{color:var(--ink3);font-size:13px}
.card{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:20px}
input,button,select{font:inherit;font-size:14px}
input{background:var(--card2);border:1px solid var(--line);border-radius:10px;color:#fff;padding:11px 13px;width:100%}
input:focus{outline:none;border-color:var(--accent)}
button{background:var(--accent);border:0;border-radius:10px;color:#1b0a04;font-weight:800;padding:11px 16px;cursor:pointer}
button.g{background:var(--card2);color:var(--ink);border:1px solid var(--line)}
button.sm{padding:7px 11px;font-size:12.5px;border-radius:8px}
button.dz{background:transparent;color:var(--bad);border:1px solid rgba(255,90,110,.4)}
.login{max-width:380px;margin:8vh auto 0}
.login h2{font-size:18px;margin-bottom:6px}.login p{color:var(--ink3);font-size:13px;margin-bottom:16px}
.login .row{display:flex;gap:10px;margin-top:14px}
.err{color:var(--bad);font-size:13px;margin-top:12px;min-height:16px}
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin:18px 0}
.stat{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:16px 18px}
.stat b{display:block;font-size:30px;font-weight:800;letter-spacing:-.03em;font-variant-numeric:tabular-nums}
.stat span{font-size:12.5px;color:var(--ink3);font-weight:600}
.stat.a b{color:var(--ok)}.stat.u b{color:var(--warn)}.stat.r b{color:var(--bad)}
.bar{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin:18px 0 14px}
.bar .grow{flex:1;min-width:180px}.bar input{width:auto}
.made{background:linear-gradient(180deg,rgba(255,107,63,.12),var(--card));border:1px solid rgba(255,107,63,.4);border-radius:14px;padding:16px 18px;margin-bottom:16px}
.made h4{font-size:13px;margin-bottom:10px;color:var(--accent);letter-spacing:.04em;text-transform:uppercase}
.made .keys{font-family:ui-monospace,Consolas,monospace;font-size:14px;font-weight:700;line-height:1.9;white-space:pre-wrap;word-break:break-all}
.tbl{background:var(--card);border:1px solid var(--line);border-radius:16px;overflow:hidden}
table{width:100%;border-collapse:collapse;font-size:13px}
th{text-align:left;padding:12px 14px;color:var(--ink3);font-weight:700;font-size:11.5px;letter-spacing:.05em;text-transform:uppercase;border-bottom:1px solid var(--line)}
td{padding:12px 14px;border-bottom:1px solid var(--line);vertical-align:middle}
tr:last-child td{border-bottom:0}
.k{font-family:ui-monospace,Consolas,monospace;font-weight:700}
.hw{font-family:ui-monospace,Consolas,monospace;font-size:11.5px;color:var(--ink2);max-width:150px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tag{font-size:11px;font-weight:800;padding:3px 10px;border-radius:999px;white-space:nowrap}
.t-unused{background:rgba(242,179,71,.16);color:var(--warn)}.t-active{background:rgba(79,194,126,.16);color:var(--ok)}.t-revoked{background:rgba(255,90,110,.16);color:var(--bad)}
.acts{display:flex;gap:6px;justify-content:flex-end;flex-wrap:wrap}
.cp{cursor:pointer;color:var(--ink3);border:0;background:none;padding:2px 4px}.cp:hover{color:var(--accent)}
.muted{color:var(--ink3);font-size:12px}.empty{padding:40px;text-align:center;color:var(--ink3)}
.hide2{display:none}
@media(max-width:700px){.stats{grid-template-columns:1fr 1fr}.hide{display:none}}
</style></head><body>
<div class="hd"><span class="mk"><svg viewBox="0 0 24 24"><path d="M21 3 3 10.5l7.5 2.5L13 21z" fill="none" stroke="#fff" stroke-width="1.7" stroke-linejoin="round"/><path d="m10.5 13 4-4" stroke="#fff" stroke-width="1.7"/></svg></span><div><h1>AstroSpeed keys</h1><div class="sub">License control panel</div></div></div>

<div id="login" class="login card">
  <h2>Sign in</h2>
  <p>Enter your admin password to manage keys.</p>
  <input id="pw" type="password" placeholder="Admin password" autofocus>
  <div class="row"><button id="go" style="flex:1" type="button">Sign in</button></div>
  <div class="err" id="loginErr"></div>
</div>

<div id="dash" class="hide2">
  <div class="stats" id="stats"></div>
  <div id="made"></div>
  <div class="bar">
    <input id="count" type="number" value="1" min="1" max="50" style="width:80px" title="How many keys">
    <input id="note" class="grow" placeholder="Note (optional) — e.g. buyer email">
    <button id="gen" type="button">+ Generate</button>
    <input id="search" class="grow" placeholder="Search key, PC or note…">
    <button class="g" id="refresh" type="button">Refresh</button>
    <button class="g" id="out" type="button">Sign out</button>
  </div>
  <div class="tbl"><table><thead><tr><th>Key</th><th>Status</th><th class="hide">PC (HWID)</th><th class="hide">Note</th><th style="text-align:right">Actions</th></tr></thead><tbody id="rows"></tbody></table></div>
</div>

<script>
(function(){
  function el(id){return document.getElementById(id);}
  function esc(s){return String(s==null?"":s).replace(/[&<>"]/g,function(c){return {"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c];});}
  function show(id,on){el(id).classList.toggle("hide2", !on);}
  var PASS="";
  try{ PASS = sessionStorage.getItem("asp") || ""; }catch(e){}

  function call(action,extra){
    return fetch("/admin",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(Object.assign({pass:PASS,action:action},extra||{}))})
      .then(function(r){return r.json();});
  }
  function copy(t){try{navigator.clipboard.writeText(t);}catch(e){}}

  var ROWS=[];
  function statusOf(r){return r.revoked?"revoked":(r.hwid?"active":"unused");}
  function draw(){
    var t=ROWS.length,a=0,u=0,rv=0;
    ROWS.forEach(function(r){var s=statusOf(r);if(s==="active")a++;else if(s==="unused")u++;else rv++;});
    el("stats").innerHTML='<div class="stat"><b>'+t+'</b><span>Total keys</span></div><div class="stat a"><b>'+a+'</b><span>Active</span></div><div class="stat u"><b>'+u+'</b><span>Unused</span></div><div class="stat r"><b>'+rv+'</b><span>Disabled</span></div>';
    var q=(el("search").value||"").toLowerCase();
    var rows=ROWS.filter(function(r){return !q||(r.key+" "+(r.hwid||"")+" "+(r.note||"")).toLowerCase().indexOf(q)>=0;});
    if(!rows.length){el("rows").innerHTML='<tr><td colspan="5" class="empty">No keys yet. Click “+ Generate”.</td></tr>';return;}
    el("rows").innerHTML=rows.map(function(r){
      var s=statusOf(r),acts='<div class="acts">';
      acts+=r.revoked?'<button class="sm g" data-a="enable" data-k="'+esc(r.key)+'">Enable</button>':'<button class="sm g" data-a="revoke" data-k="'+esc(r.key)+'">Disable</button>';
      if(r.hwid)acts+='<button class="sm g" data-a="unbind" data-k="'+esc(r.key)+'">Reset PC</button>';
      acts+='<button class="sm dz" data-a="delete" data-k="'+esc(r.key)+'">Delete</button></div>';
      return '<tr><td><span class="k">'+esc(r.key)+'</span> <button class="cp" data-cp="'+esc(r.key)+'">⧉</button></td><td><span class="tag t-'+s+'">'+s+'</span></td><td class="hide"><div class="hw">'+(r.hwid?esc(r.hwid):'—')+'</div></td><td class="hide">'+(r.note?esc(r.note):'—')+'</td><td>'+acts+'</td></tr>';
    }).join("");
    [].forEach.call(document.querySelectorAll("[data-cp]"),function(b){b.onclick=function(){copy(b.getAttribute("data-cp"));b.textContent="✓";setTimeout(function(){b.textContent="⧉";},900);};});
    [].forEach.call(document.querySelectorAll("[data-a]"),function(b){b.onclick=function(){act(b.getAttribute("data-k"),b.getAttribute("data-a"));};});
  }
  function load(cb){
    call("list").then(function(d){
      if(!d||d.ok===false){ if(cb)cb(d&&d.reason?d.reason:"Wrong password"); return; }
      ROWS=(d.rows||[]).sort(function(a,b){return (b.createdAt||0)-(a.createdAt||0);});
      show("login",false); show("dash",true); draw();
      if(cb)cb(null);
    }).catch(function(){ if(cb)cb("Couldn't reach the server."); });
  }
  function doLogin(){
    PASS=el("pw").value;
    el("loginErr").textContent="Checking…";
    load(function(err){ if(err){ el("loginErr").textContent=err; } else { try{sessionStorage.setItem("asp",PASS);}catch(e){} el("loginErr").textContent=""; } });
  }
  function gen(){
    call("generate",{count:el("count").value,note:el("note").value}).then(function(d){
      if(!d.ok){alert(d.reason||"Error");return;}
      el("made").innerHTML='<div class="made"><h4>New key'+(d.made.length>1?"s":"")+' — copy and send to the buyer</h4><div class="keys">'+d.made.map(esc).join("\n")+'</div><div style="margin-top:12px"><button class="sm" id="cpall" type="button">Copy all</button></div></div>';
      el("cpall").onclick=function(){copy(d.made.join("\n"));el("cpall").textContent="Copied ✓";};
      el("note").value=""; load();
    });
  }
  function act(key,a){
    if(a==="delete"&&!confirm("Delete "+key+" permanently?"))return;
    if(a==="revoke"&&!confirm("Disable "+key+"? The app locks on that PC next launch."))return;
    if(a==="unbind"&&!confirm("Reset "+key+" so it can activate on a different PC?"))return;
    call(a,{key:key}).then(function(d){if(d&&d.ok===false)alert(d.reason||"Error");load();});
  }

  el("go").onclick=doLogin;
  el("pw").addEventListener("keydown",function(e){if(e.key==="Enter")doLogin();});
  el("gen").onclick=gen; el("refresh").onclick=function(){load();}; el("search").addEventListener("input",draw);
  el("out").onclick=function(){try{sessionStorage.removeItem("asp");}catch(e){}location.reload();};
  if(PASS){ load(function(err){ if(err){ show("login",true); show("dash",false); } }); }
})();
</script></body></html>`;
