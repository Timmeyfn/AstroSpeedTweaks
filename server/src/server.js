// ════════════════════════════════════════════════════════════════════════════
//  AstroSpeed server — ONE file for everything. Paste this whole file into your
//  Cloudflare worker and press Deploy. You never need to edit it.
//
//  It runs:
//    /                 the sales website
//    /buy  /thanks     Stripe payments — the buyer gets their key on screen instantly
//    /download         the app download (you upload new versions in the admin page)
//    /version.json     tells the app when there's an update
//    /activate /check  key activation, locked to one PC
//    /pack             live tweaks you add in the admin page
//    /admin            YOUR control panel: keys, sales, releases, price, live tweaks
//
//  Worker settings it needs (Settings tab of the worker):
//    Binding   KV namespace   LICENSES          (where everything is stored)
//    Variable  ADMIN_PASS     your password     (at least 10 characters)
//    Secret    STRIPE_SECRET_KEY  sk_live_...   (only when you want to take payments)
//    Secret    STRIPE_WEBHOOK_SECRET whsec_...  (optional backup, see the guide)
// ════════════════════════════════════════════════════════════════════════════

const SERVER_VERSION = "2.0";
// Where the app was downloaded from before you upload a version in the admin page.
const LEGACY_DOWNLOAD = "https://astrospeedtweaks.dupelication.workers.dev/AstroSpeed%20Tweaks.exe";
const EXE_NAME = "AstroSpeed Tweaks.exe";
const MIN_PASS = 10;

const DEFAULT_SETTINGS = {
  productName: "AstroSpeed Tweaks",
  price: 500, // in cents: 500 = $5.00
  currency: "aud",
  supportEmail: "",
};

const CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
  "Access-Control-Allow-Headers": "Content-Type, X-Admin-Pass",
};

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const path = url.pathname.replace(/\/+$/, "") || "/";
    if (request.method === "OPTIONS") return new Response(null, { headers: CORS });
    try {
      switch (path) {
        case "/":
        case "/index.html":
          return page(SITE_HTML);
        case "/config.json":
          return await publicConfig(env);
        case "/version.json":
          return await versionInfo(env, url);
        case "/download":
          return await download(env);
        case "/activate":
          return await activate(request, env);
        case "/check":
          return await check(request, env);
        case "/pack":
          return await packData(env);
        case "/buy":
          return await buy(env, url);
        case "/thanks":
          return await thanks(env, url);
        case "/stripe-webhook":
          return await stripeWebhook(request, env);
        case "/admin":
          return request.method === "GET" ? page(ADMIN_HTML) : await adminApi(request, env);
        case "/admin/upload":
          return await adminUpload(request, env, url);
        case "/health":
          return json(health(env));
      }
      return new Response("Not found", { status: 404, headers: CORS });
    } catch (e) {
      return json({ ok: false, reason: "Server error: " + (e && e.message ? e.message : String(e)) }, 500);
    }
  },
};

// ───────────────────────── helpers ─────────────────────────

function json(obj, status = 200) {
  return new Response(JSON.stringify(obj), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "no-store", ...CORS },
  });
}
function page(html, status = 200) {
  return new Response(html, { status, headers: { "Content-Type": "text/html; charset=utf-8", "Cache-Control": "no-store" } });
}
function esc(s) {
  return String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}
function normKey(k) {
  return String(k || "").toUpperCase().replace(/[^A-Z0-9]/g, "").replace(/(.{5})/g, "$1-").replace(/-$/, "");
}
function isKeyName(name) {
  return /^ASTRO(-[A-Z0-9]{5}){3}$/.test(name);
}
function newKey() {
  const abc = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"; // no 0/O/1/I/L
  const a = new Uint8Array(15);
  crypto.getRandomValues(a);
  let out = "ASTRO";
  for (let i = 0; i < 15; i++) {
    if (i % 5 === 0) out += "-";
    out += abc[a[i] % abc.length];
  }
  return out;
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
function newerVersion(a, b) {
  const x = String(a).split(".").map(Number), y = String(b).split(".").map(Number);
  for (let i = 0; i < Math.max(x.length, y.length); i++) {
    const p = x[i] || 0, q = y[i] || 0;
    if (p !== q) return p > q;
  }
  return false;
}

function health(env) {
  const pass = typeof env.ADMIN_PASS === "string" ? env.ADMIN_PASS.trim() : "";
  return {
    server: SERVER_VERSION,
    storage: !!env.LICENSES,
    password: pass.length > 0,
    passwordStrong: pass.length >= MIN_PASS,
    stripe: typeof env.STRIPE_SECRET_KEY === "string" && env.STRIPE_SECRET_KEY.trim().startsWith("sk_"),
    stripeTest: typeof env.STRIPE_SECRET_KEY === "string" && env.STRIPE_SECRET_KEY.trim().startsWith("sk_test_"),
    webhook: typeof env.STRIPE_WEBHOOK_SECRET === "string" && env.STRIPE_WEBHOOK_SECRET.trim().startsWith("whsec_"),
  };
}
function needStorage(env) {
  if (!env.LICENSES) throw new Error("the LICENSES storage isn't connected to this worker (Settings > Bindings)");
}
async function getSettings(env) {
  const s = (env.LICENSES && (await env.LICENSES.get("cfg:settings", "json"))) || {};
  return { ...DEFAULT_SETTINGS, ...s };
}
async function getRelease(env) {
  return (env.LICENSES && (await env.LICENSES.get("cfg:release", "json"))) || null;
}
function money(cents, currency) {
  const v = (Number(cents) || 0) / 100;
  const sym = { aud: "$", usd: "$", nzd: "$", cad: "$", gbp: "£", eur: "€" }[String(currency).toLowerCase()] || "";
  return sym + (v % 1 === 0 ? v.toFixed(0) : v.toFixed(2)) + (sym === "$" ? " " + String(currency).toUpperCase() : "");
}

// ───────────────────────── public: config, version, download, pack ─────────────────────────

async function publicConfig(env) {
  const s = await getSettings(env);
  const h = health(env);
  const rel = await getRelease(env);
  return json({
    productName: s.productName,
    price: s.price,
    currency: s.currency,
    priceLabel: money(s.price, s.currency),
    priceShort: money(s.price, s.currency).replace(/ [A-Z]{3}$/, ""),
    currencyCode: String(s.currency).toUpperCase(),
    buyEnabled: h.stripe && h.storage && s.price >= 50,
    supportEmail: s.supportEmail,
    version: rel ? rel.version : "",
  });
}

async function versionInfo(env, url) {
  const rel = await getRelease(env);
  if (!rel) return json({ version: "", notes: "", download: "" });
  return json({
    version: rel.version,
    title: "Version " + rel.version,
    notes: rel.notes || "",
    published: rel.publishedAt ? new Date(rel.publishedAt).toISOString() : "",
    download: url.origin + "/download?v=" + encodeURIComponent(rel.version),
  });
}

async function download(env) {
  const rel = await getRelease(env);
  if (!rel || !rel.fileKey) return Response.redirect(LEGACY_DOWNLOAD, 302);
  const obj = await env.LICENSES.getWithMetadata(rel.fileKey, { type: "stream" });
  if (!obj || !obj.value) return Response.redirect(LEGACY_DOWNLOAD, 302);
  const headers = {
    "Content-Type": "application/octet-stream",
    "Content-Disposition": 'attachment; filename="' + EXE_NAME + '"',
    "Cache-Control": "no-store",
    ...CORS,
  };
  if (obj.metadata && obj.metadata.size) headers["Content-Length"] = String(obj.metadata.size);
  return new Response(obj.value, { headers });
}

async function packData(env) {
  const p = (env.LICENSES && (await env.LICENSES.get("cfg:pack", "json"))) || null;
  if (!p) return json({ found: false, json: "", bats: {} });
  let js = p.json || "";
  if (p.message && !js.trim()) js = JSON.stringify({ message: p.message });
  else if (p.message) {
    try {
      const o = JSON.parse(js.split("\n").filter((l) => !l.trim().startsWith("//")).join("\n"));
      o.message = p.message;
      js = JSON.stringify(o);
    } catch (e) {
      /* leave the JSON as typed; the app reports the mistake */
    }
  }
  const bats = p.bats || {};
  return json({ found: !!(js.trim() || Object.keys(bats).length), json: js, bats });
}

// ───────────────────────── activation ─────────────────────────

async function activate(request, env) {
  needStorage(env);
  const { key, hwid } = await request.json().catch(() => ({}));
  const k = normKey(key);
  if (!k || !hwid) return json({ ok: false, code: "missing", reason: "Enter your key." });
  const rec = await env.LICENSES.get(k, "json");
  if (!rec) return json({ ok: false, code: "invalid", reason: "That key isn't valid. Check it was copied exactly." });
  if (rec.revoked) return json({ ok: false, code: "disabled", reason: "This key has been disabled." });
  if (rec.hwid && rec.hwid !== hwid)
    return json({ ok: false, code: "other_pc", reason: "This key is already activated on another PC." });
  if (!rec.hwid) {
    rec.hwid = hwid;
    rec.activatedAt = Date.now();
    await env.LICENSES.put(k, JSON.stringify(rec));
  }
  return json({ ok: true, key: k });
}

async function check(request, env) {
  needStorage(env);
  const { key, hwid } = await request.json().catch(() => ({}));
  const k = normKey(key);
  const rec = await env.LICENSES.get(k, "json");
  if (!rec) return json({ ok: false, reason: "invalid" });
  if (rec.revoked) return json({ ok: false, reason: "disabled" });
  if (rec.hwid !== hwid) return json({ ok: false, reason: rec.hwid ? "other_pc" : "invalid" });
  return json({ ok: true });
}

// ───────────────────────── payments (Stripe) ─────────────────────────

async function stripe(env, method, path, form) {
  const res = await fetch("https://api.stripe.com/v1/" + path, {
    method,
    headers: {
      Authorization: "Bearer " + env.STRIPE_SECRET_KEY.trim(),
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: form ? new URLSearchParams(form).toString() : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error((data.error && data.error.message) || "Stripe answered " + res.status);
  return data;
}

async function buy(env, url) {
  const h = health(env);
  const s = await getSettings(env);
  if (!h.stripe || !h.storage || s.price < 50) {
    return page(simplePage("Payments aren't open yet", "<p>Buying keys isn't switched on yet. Please check back soon.</p>", url.origin));
  }
  const session = await stripe(env, "POST", "checkout/sessions", {
    mode: "payment",
    "line_items[0][quantity]": "1",
    "line_items[0][price_data][currency]": s.currency,
    "line_items[0][price_data][unit_amount]": String(s.price),
    "line_items[0][price_data][product_data][name]": s.productName + " — full version key",
    "line_items[0][price_data][product_data][description]": "One key, locked to one PC. Never expires.",
    success_url: url.origin + "/thanks?session_id={CHECKOUT_SESSION_ID}",
    cancel_url: url.origin + "/#pricing",
    "metadata[product]": "astrospeed",
  });
  return Response.redirect(session.url, 303);
}

// The key for an order is worked out from the order id itself (HMAC with your Stripe secret),
// so the thanks page and the webhook always arrive at the SAME key, even if they run at once.
async function saleKey(env, sessionId) {
  const k = await crypto.subtle.importKey("raw", new TextEncoder().encode("astrospeed-sale:" + env.STRIPE_SECRET_KEY.trim()), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const sig = new Uint8Array(await crypto.subtle.sign("HMAC", k, new TextEncoder().encode(sessionId)));
  const abc = "ABCDEFGHJKMNPQRSTUVWXYZ23456789";
  let out = "ASTRO";
  for (let i = 0; i < 15; i++) {
    if (i % 5 === 0) out += "-";
    out += abc[sig[i] % abc.length];
  }
  return out;
}

// Creates the key for a paid order once; calling it again returns the same key.
async function keyForSale(env, session) {
  const existing = await env.LICENSES.get("sale:" + session.id, "json");
  if (existing && existing.key) return existing.key;
  const email = (session.customer_details && session.customer_details.email) || session.customer_email || "";
  const key = await saleKey(env, session.id);
  const already = await env.LICENSES.get(key, "json");
  if (already) {
    // the other path (webhook / thanks page) got here first: just make sure the sale is recorded
    await env.LICENSES.put("sale:" + session.id, JSON.stringify({ key, email, amount: session.amount_total || 0, currency: session.currency || "", createdAt: already.createdAt || Date.now() }));
    return key;
  }
  await env.LICENSES.put(
    key,
    JSON.stringify({ createdAt: Date.now(), hwid: null, revoked: false, note: email || "Stripe sale", source: "stripe", sale: session.id }),
  );
  await env.LICENSES.put(
    "sale:" + session.id,
    JSON.stringify({ key, email, amount: session.amount_total || 0, currency: session.currency || "", createdAt: Date.now() }),
  );
  return key;
}

async function thanks(env, url) {
  const id = url.searchParams.get("session_id") || "";
  if (!/^cs_[A-Za-z0-9_]{10,200}$/.test(id)) return page(simplePage("Order not found", "<p>That link isn't a valid order.</p>", url.origin), 404);
  needStorage(env);
  const known = await env.LICENSES.get("sale:" + id, "json");
  let key = known && known.key;
  if (!key) {
    if (!health(env).stripe) return page(simplePage("Order not found", "<p>Payments aren't set up on this server.</p>", url.origin), 404);
    const session = await stripe(env, "GET", "checkout/sessions/" + encodeURIComponent(id));
    if (session.payment_status !== "paid") {
      return page(
        simplePage(
          "Payment still processing",
          '<p>Your payment hasn\'t finished yet. This page will refresh by itself.</p><script>setTimeout(function(){location.reload()},4000)</script>',
          url.origin,
        ),
      );
    }
    key = await keyForSale(env, session);
  }
  return page(THANKS_HTML.split("{{KEY}}").join(esc(key)).split("{{ORIGIN}}").join(esc(url.origin)));
}

async function hmacHex(secret, text) {
  const k = await crypto.subtle.importKey("raw", new TextEncoder().encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const sig = await crypto.subtle.sign("HMAC", k, new TextEncoder().encode(text));
  return [...new Uint8Array(sig)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

async function stripeWebhook(request, env) {
  const secret = (env.STRIPE_WEBHOOK_SECRET || "").trim();
  if (!secret) return json({ ok: false, reason: "webhook secret not set" }, 400);
  const body = await request.text();
  const header = request.headers.get("Stripe-Signature") || "";
  const parts = Object.fromEntries(header.split(",").map((p) => p.split("=")).filter((p) => p.length === 2));
  const sigs = header.split(",").filter((p) => p.startsWith("v1=")).map((p) => p.slice(3));
  const t = Number(parts.t || 0);
  if (!t || Math.abs(Date.now() / 1000 - t) > 600) return json({ ok: false, reason: "bad timestamp" }, 400);
  const expected = await hmacHex(secret, t + "." + body);
  if (!sigs.includes(expected)) return json({ ok: false, reason: "bad signature" }, 400);
  const event = JSON.parse(body);
  if (event.type === "checkout.session.completed" || event.type === "checkout.session.async_payment_succeeded") {
    const session = event.data && event.data.object;
    if (session && session.payment_status === "paid") {
      needStorage(env);
      await keyForSale(env, session);
    }
  }
  return json({ ok: true });
}

// ───────────────────────── admin ─────────────────────────

function passOK(env, given) {
  const want = typeof env.ADMIN_PASS === "string" ? env.ADMIN_PASS.trim() : "";
  const got = String(given || "").trim();
  if (want.length < MIN_PASS || got.length !== want.length) return false;
  let diff = 0;
  for (let i = 0; i < want.length; i++) diff |= want.charCodeAt(i) ^ got.charCodeAt(i);
  return diff === 0;
}
async function authFail(env) {
  const h = health(env);
  if (!h.password)
    return json({ ok: false, code: "nopass", reason: "No ADMIN_PASS is set on this worker yet. Add it under Settings > Variables and Secrets." }, 401);
  if (!h.passwordStrong)
    return json(
      { ok: false, code: "weak", reason: "Your ADMIN_PASS is too short. Change it to at least " + MIN_PASS + " characters in the worker's Settings, then try again." },
      401,
    );
  await sleep(700); // slows down guessing
  return json({ ok: false, code: "wrong", reason: "Wrong password." }, 401);
}

async function listKeys(env) {
  const rows = [];
  let cursor;
  do {
    const page = await env.LICENSES.list({ prefix: "ASTRO-", cursor, limit: 1000 });
    for (const it of page.keys) {
      if (!isKeyName(it.name)) continue;
      const rec = await env.LICENSES.get(it.name, "json");
      if (rec) rows.push({ key: it.name, ...rec });
    }
    cursor = page.list_complete ? null : page.cursor;
  } while (cursor && rows.length < 5000);
  rows.sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));
  return rows;
}
async function listSales(env) {
  const rows = [];
  let cursor;
  do {
    const page = await env.LICENSES.list({ prefix: "sale:", cursor, limit: 1000 });
    for (const it of page.keys) {
      const rec = await env.LICENSES.get(it.name, "json");
      if (rec) rows.push({ id: it.name.slice(5), ...rec });
    }
    cursor = page.list_complete ? null : page.cursor;
  } while (cursor && rows.length < 5000);
  rows.sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));
  return rows;
}

async function adminApi(request, env) {
  const body = await request.json().catch(() => ({}));
  if (!passOK(env, body.pass)) return authFail(env);
  const h = health(env);
  const action = body.action;
  if (action === "login" || action === "status") {
    return json({ ok: true, health: h, settings: await getSettings(env), release: await getRelease(env) });
  }
  needStorage(env);

  if (action === "keys") return json({ ok: true, rows: await listKeys(env) });
  if (action === "generate") {
    const n = Math.min(Math.max(parseInt(body.count, 10) || 1, 1), 50);
    const made = [];
    for (let i = 0; i < n; i++) {
      const key = newKey();
      await env.LICENSES.put(key, JSON.stringify({ createdAt: Date.now(), hwid: null, revoked: false, note: String(body.note || "").slice(0, 200), source: "admin" }));
      made.push(key);
    }
    return json({ ok: true, made });
  }
  if (["revoke", "enable", "unbind", "delete"].includes(action)) {
    const k = normKey(body.key);
    const rec = await env.LICENSES.get(k, "json");
    if (!rec) return json({ ok: false, reason: "That key doesn't exist." });
    if (action === "delete") {
      await env.LICENSES.delete(k);
      return json({ ok: true });
    }
    if (action === "revoke") rec.revoked = true;
    if (action === "enable") rec.revoked = false;
    if (action === "unbind") {
      rec.hwid = null;
      rec.activatedAt = null;
    }
    await env.LICENSES.put(k, JSON.stringify(rec));
    return json({ ok: true });
  }
  if (action === "note") {
    const k = normKey(body.key);
    const rec = await env.LICENSES.get(k, "json");
    if (!rec) return json({ ok: false, reason: "That key doesn't exist." });
    rec.note = String(body.note || "").slice(0, 200);
    await env.LICENSES.put(k, JSON.stringify(rec));
    return json({ ok: true });
  }
  if (action === "sales") return json({ ok: true, rows: await listSales(env) });

  if (action === "saveSettings") {
    const cur = await getSettings(env);
    const s = body.settings || {};
    const price = Math.round(Number(s.price));
    const next = {
      productName: String(s.productName || cur.productName).slice(0, 80),
      price: isFinite(price) && price >= 0 ? price : cur.price,
      currency: /^[a-z]{3}$/.test(String(s.currency || "").toLowerCase()) ? String(s.currency).toLowerCase() : cur.currency,
      supportEmail: String(s.supportEmail || "").slice(0, 120),
    };
    if (next.price > 0 && next.price < 50) return json({ ok: false, reason: "The price has to be at least 0.50 (Stripe's minimum)." });
    await env.LICENSES.put("cfg:settings", JSON.stringify(next));
    return json({ ok: true, settings: next });
  }

  if (action === "publish") {
    const version = String(body.version || "").trim().replace(/^v/i, "");
    if (!/^\d+(\.\d+){1,3}$/.test(version)) return json({ ok: false, reason: "Version must look like 1.9.0" });
    const fileKey = "file:app:" + version;
    const meta = await env.LICENSES.get("filemeta:app:" + version, "json");
    if (!meta) return json({ ok: false, reason: "Upload the .exe for this version first." });
    const live = await getRelease(env);
    if (live && !newerVersion(version, live.version) && !body.force)
      return json({ ok: false, code: "not_newer", reason: "Version " + version + " isn't higher than the live " + live.version + ", so nobody would get it." });
    const rel = { version, notes: String(body.notes || "").slice(0, 4000), fileKey, size: meta.size || 0, publishedAt: Date.now() };
    await env.LICENSES.put("cfg:release", JSON.stringify(rel));
    // tidy away older uploads so storage doesn't fill up (keep the live one and this one)
    const old = await env.LICENSES.list({ prefix: "file:app:" });
    for (const it of old.keys) if (it.name !== fileKey) await env.LICENSES.delete(it.name);
    const oldMeta = await env.LICENSES.list({ prefix: "filemeta:app:" });
    for (const it of oldMeta.keys) if (it.name !== "filemeta:app:" + version) await env.LICENSES.delete(it.name);
    return json({ ok: true, release: rel });
  }

  if (action === "getPack") {
    return json({ ok: true, pack: (await env.LICENSES.get("cfg:pack", "json")) || { message: "", json: "", bats: {} } });
  }
  if (action === "savePack") {
    const p = body.pack || {};
    const bats = {};
    let total = 0;
    for (const [name, text] of Object.entries(p.bats || {})) {
      const clean = String(name).replace(/[^A-Za-z0-9 _.\-]/g, "").slice(0, 60);
      if (!clean) continue;
      const fn = /\.(bat|cmd)$/i.test(clean) ? clean : clean + ".bat";
      const t = String(text || "").slice(0, 256 * 1024);
      total += t.length;
      bats[fn] = t;
    }
    if (Object.keys(bats).length > 30) return json({ ok: false, reason: "That's more than 30 .bat files." });
    if (total > 4 * 1024 * 1024) return json({ ok: false, reason: "Those .bat files are too big in total." });
    const pack = { message: String(p.message || "").slice(0, 400), json: String(p.json || "").slice(0, 512 * 1024), bats, savedAt: Date.now() };
    await env.LICENSES.put("cfg:pack", JSON.stringify(pack));
    return json({ ok: true, pack });
  }

  return json({ ok: false, reason: "Unknown action" }, 400);
}

async function adminUpload(request, env, url) {
  if (request.method !== "POST") return json({ ok: false, reason: "POST only" }, 405);
  if (!passOK(env, request.headers.get("X-Admin-Pass"))) return authFail(env);
  needStorage(env);
  const version = String(url.searchParams.get("version") || "").trim().replace(/^v/i, "");
  if (!/^\d+(\.\d+){1,3}$/.test(version)) return json({ ok: false, reason: "Version must look like 1.9.0" });
  const buf = await request.arrayBuffer();
  if (buf.byteLength < 1024 * 1024) return json({ ok: false, reason: "That file is too small to be the app." });
  if (buf.byteLength > 24 * 1024 * 1024) return json({ ok: false, reason: "That file is over 24 MB, which is too big to store." });
  const head = new Uint8Array(buf, 0, 2);
  if (head[0] !== 0x4d || head[1] !== 0x5a) return json({ ok: false, reason: "That isn't a Windows .exe file." });
  await env.LICENSES.put("file:app:" + version, buf, { metadata: { size: buf.byteLength, uploadedAt: Date.now() } });
  await env.LICENSES.put("filemeta:app:" + version, JSON.stringify({ size: buf.byteLength, uploadedAt: Date.now() }));
  return json({ ok: true, size: buf.byteLength });
}

function simplePage(title, bodyHtml, origin) {
  return SIMPLE_HTML.split("{{TITLE}}").join(esc(title)).split("{{BODY}}").join(bodyHtml).split("{{ORIGIN}}").join(esc(origin || ""));
}

// ───────────────────────── pages (filled in by build_server.py) ─────────────────────────
const SITE_HTML = __SITE_HTML__;
const ADMIN_HTML = __ADMIN_HTML__;
const THANKS_HTML = __THANKS_HTML__;
const SIMPLE_HTML = __SIMPLE_HTML__;
