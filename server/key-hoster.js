// ════════════════════════════════════════════════════════════════════════════
//  AstroSpeed KEY HOSTER (rapid-forest) — keys only. Paste this whole file into your
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
//    Secret    ADMIN_PASS     your password     (at least 12 characters, longer is better)
//    Secret    STRIPE_SECRET_KEY  sk_live_...   (only when you want to take payments)
//    Secret    STRIPE_WEBHOOK_SECRET whsec_...  (recommended: saves keys even if a buyer closes the tab,
//                                               and disables keys for refunds/chargebacks)
// ════════════════════════════════════════════════════════════════════════════

const SERVER_VERSION = "2.0";
// "all" = website + shop + keys (astrospeed worker). "keys" = key hoster only (rapid-forest worker).
const ROLE = "keys";
const KEY_ROUTES = new Set(["/activate", "/check", "/health", "/admin", "/admin/upload"]);
// Where the app was downloaded from before you upload a version in the admin page.
const LEGACY_DOWNLOAD = "https://astrospeedtweaks.dupelication.workers.dev/AstroSpeed%20Tweaks.exe";
const EXE_NAME = "AstroSpeed Tweaks.exe";
const MIN_PASS = 12;

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
    if (ROLE === "keys" && !KEY_ROUTES.has(path))
      return new Response("AstroSpeed key server is running.", { headers: { "Content-Type": "text/plain; charset=utf-8", ...CORS } });
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
          return await download(env, url);
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
          return request.method === "GET" ? adminPage() : await adminApi(request, env, request);
        case "/admin/upload":
          return await adminUpload(request, env, url);
        case "/health":
          return json(health(env));
      }
      return new Response("Not found", { status: 404, headers: CORS });
    } catch (e) {
      const msg = e && e.message ? e.message : String(e);
      console.log("error on " + path + ": " + msg);
      if (request.method === "GET" && (path === "/thanks" || path === "/buy"))
        return page(simplePage("Something went wrong", "<p>We couldn't load this page just now. Your payment (if you made one) is safe. Please refresh in a minute" + (path === "/thanks" ? " — this same link will show your key." : ".") + "</p>", url.origin), 500);
      // only setup problems are spelled out; anything else stays generic
      const setup = /LICENSES storage/.test(msg);
      return json({ ok: false, reason: setup ? "Server error: " + msg : "Server error. Please try again." }, 500);
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
    minPassword: MIN_PASS,
  };
}
// Every key record is also stored as metadata, so the admin list can read it in one go.
function keyMeta(rec) {
  return {
    createdAt: rec.createdAt || 0, activatedAt: rec.activatedAt || 0, hwid: rec.hwid || "", revoked: !!rec.revoked,
    note: String(rec.note || "").slice(0, 200), source: rec.source || "", sale: rec.sale || "",
    trialMs: rec.trialMs || 0, expiresAt: rec.expiresAt || 0,
  };
}
async function putKey(env, key, rec) {
  await env.LICENSES.put(key, JSON.stringify(rec), { metadata: keyMeta(rec) });
}
function adminPage() {
  return new Response(ADMIN_HTML, {
    headers: { "Content-Type": "text/html; charset=utf-8", "Cache-Control": "no-store", "X-Frame-Options": "DENY", "Referrer-Policy": "no-referrer" },
  });
}
// Slows down password guessing (per Cloudflare server; real protection is a long password).
const FAILS = new Map();
function throttled(ip) {
  const f = FAILS.get(ip);
  return !!(f && f.n >= 8 && Date.now() - f.t < 15 * 60 * 1000);
}
function noteFail(ip) {
  const f = FAILS.get(ip) || { n: 0, t: Date.now() };
  if (Date.now() - f.t > 15 * 60 * 1000) { f.n = 0; f.t = Date.now(); }
  f.n++;
  FAILS.set(ip, f);
  if (FAILS.size > 5000) FAILS.clear();
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

async function download(env, url) {
  const rel = await getRelease(env);
  const want = String(url.searchParams.get("v") || "").replace(/[^0-9.]/g, "");
  if (!rel && !want) return Response.redirect(LEGACY_DOWNLOAD, 302); // nothing published yet
  const version = want || rel.version;
  const obj = await env.LICENSES.getWithMetadata("file:app:" + version, { type: "stream" });
  if (!obj || !obj.value) {
    // never hand out a different file than the one asked for: the app would install the wrong version
    return new Response("This download isn't available right now. Please try again in a minute.", { status: 503, headers: { "Retry-After": "60", ...CORS } });
  }
  const headers = {
    "Content-Type": "application/octet-stream",
    "Content-Disposition": 'attachment; filename="' + EXE_NAME + '"',
    "Cache-Control": "no-store",
    "X-AstroSpeed-Version": version,
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
  if (rec.expiresAt && Date.now() >= rec.expiresAt) return json({ ok: false, code: "expired", reason: "This trial key has ended." });
  if (rec.hwid && rec.hwid !== hwid)
    return json({ ok: false, code: "other_pc", reason: "This key is already activated on another PC." });
  if (!rec.hwid) {
    rec.hwid = hwid;
    rec.activatedAt = Date.now();
    if (rec.trialMs && !rec.expiresAt) rec.expiresAt = Date.now() + rec.trialMs; // a trial's clock starts now
    await putKey(env, k, rec);
  }
  return json({ ok: true, key: k, expiresAt: rec.expiresAt || 0 });
}

async function check(request, env) {
  needStorage(env);
  const { key, hwid } = await request.json().catch(() => ({}));
  const k = normKey(key);
  const rec = await env.LICENSES.get(k, "json");
  if (!rec) return json({ ok: false, reason: "invalid" });
  if (rec.revoked) return json({ ok: false, reason: "disabled" });
  if (rec.expiresAt && Date.now() >= rec.expiresAt) return json({ ok: false, reason: "expired" });
  if (!rec.hwid) return json({ ok: false, reason: "not_activated" }); // e.g. just reset, or storage still catching up
  if (rec.hwid !== hwid) return json({ ok: false, reason: "other_pc" });
  return json({ ok: true, expiresAt: rec.expiresAt || 0 });
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

function isOurPaidCheckout(s) {
  return !!s && s.mode === "payment" && (s.payment_status === "paid" || s.payment_status === "no_payment_required") && s.metadata && s.metadata.product === "astrospeed";
}

// Creates the key for a paid order once; calling it again returns the same key.
async function keyForSale(env, session) {
  const existing = await env.LICENSES.get("sale:" + session.id, "json");
  if (existing && existing.key) return existing.key;
  const email = (session.customer_details && session.customer_details.email) || session.customer_email || "";
  const key = await saleKey(env, session.id);
  const pi = typeof session.payment_intent === "string" ? session.payment_intent : (session.payment_intent && session.payment_intent.id) || "";
  const sale = { key, email, amount: session.amount_total || 0, currency: session.currency || "", pi, createdAt: Date.now() };
  const already = await env.LICENSES.get(key, "json");
  if (already) {
    // the other path (webhook / thanks page) got here first: just make sure the sale is recorded
    sale.createdAt = already.createdAt || sale.createdAt;
    await env.LICENSES.put("sale:" + session.id, JSON.stringify(sale), { metadata: sale });
    return key;
  }
  await putKey(env, key, { createdAt: Date.now(), hwid: null, revoked: false, note: email || "Stripe sale", source: "stripe", sale: session.id });
  await env.LICENSES.put("sale:" + session.id, JSON.stringify(sale), { metadata: sale });
  if (pi) await env.LICENSES.put("pi:" + pi, session.id);
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
    if (!session || session.mode !== "payment" || !session.metadata || session.metadata.product !== "astrospeed")
      return page(simplePage("Order not found", "<p>That link isn't an AstroSpeed order.</p>", url.origin), 404);
    if (!isOurPaidCheckout(session)) {
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
  const obj = event.data && event.data.object;
  if (event.type === "checkout.session.completed" || event.type === "checkout.session.async_payment_succeeded") {
    if (isOurPaidCheckout(obj)) {
      needStorage(env);
      await keyForSale(env, obj);
    }
  }
  if ((event.type === "charge.refunded" && obj && obj.refunded) || event.type === "charge.dispute.created") {
    const pi = obj && (typeof obj.payment_intent === "string" ? obj.payment_intent : obj.payment_intent && obj.payment_intent.id);
    const sid = pi && (await env.LICENSES.get("pi:" + pi));
    const sale = sid && (await env.LICENSES.get("sale:" + sid, "json"));
    if (sale && sale.key) {
      const rec = await env.LICENSES.get(sale.key, "json");
      if (rec && !rec.revoked) {
        rec.revoked = true;
        rec.note = (rec.note ? rec.note + " · " : "") + (event.type === "charge.dispute.created" ? "chargeback" : "refunded");
        await putKey(env, sale.key, rec);
      }
    }
  }
  return json({ ok: true });
}

// ───────────────────────── admin ─────────────────────────

async function sha256(text) {
  return new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text)));
}
async function passOK(env, given, request) {
  const ip = (request && request.headers.get("CF-Connecting-IP")) || "local";
  if (throttled(ip)) return false;
  const want = typeof env.ADMIN_PASS === "string" ? env.ADMIN_PASS.trim() : "";
  if (want.length < MIN_PASS) return false;
  const a = await sha256(want), b = await sha256(String(given || "").trim());
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a[i] ^ b[i];
  if (diff !== 0) noteFail(ip);
  return diff === 0;
}
async function authFail(env, request) {
  const h = health(env);
  const ip = (request && request.headers.get("CF-Connecting-IP")) || "local";
  if (throttled(ip)) return json({ ok: false, code: "throttled", reason: "Too many wrong passwords. Wait 15 minutes and try again." }, 429);
  if (!h.password)
    return json({ ok: false, code: "nopass", reason: "No ADMIN_PASS is set on this worker yet. Add it under Settings > Variables and Secrets." }, 401);
  if (!h.passwordStrong)
    return json(
      { ok: false, code: "weak", reason: "Your ADMIN_PASS is too short. Change it to at least " + MIN_PASS + " characters (a long random one is best) in the worker's Settings, then try again." },
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
      if (it.metadata && "createdAt" in it.metadata) rows.push({ key: it.name, ...it.metadata });
      else {
        // made by an older server version: read it once and add metadata for next time
        const rec = await env.LICENSES.get(it.name, "json");
        if (rec) { rows.push({ key: it.name, ...rec }); if (rows.length < 300) await putKey(env, it.name, rec); }
      }
    }
    cursor = page.list_complete ? null : page.cursor;
  } while (cursor && rows.length < 20000);
  rows.sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));
  return rows;
}
async function listSales(env) {
  const rows = [];
  let cursor;
  do {
    const page = await env.LICENSES.list({ prefix: "sale:", cursor, limit: 1000 });
    for (const it of page.keys) {
      const rec = it.metadata && it.metadata.key ? it.metadata : await env.LICENSES.get(it.name, "json");
      if (rec) rows.push({ id: it.name.slice(5), ...rec });
    }
    cursor = page.list_complete ? null : page.cursor;
  } while (cursor && rows.length < 20000);
  rows.sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));
  return rows;
}

// Finds paid Stripe orders that never got a key (e.g. buyer closed the tab and there's no webhook).
async function syncSales(env) {
  if (!health(env).stripe) throw new Error("Stripe isn't connected (no STRIPE_SECRET_KEY).");
  const list = await stripe(env, "GET", "checkout/sessions?limit=100&status=complete");
  let made = 0, checked = 0;
  for (const sess of list.data || []) {
    if (!isOurPaidCheckout(sess)) continue;
    checked++;
    const known = await env.LICENSES.get("sale:" + sess.id, "json");
    if (known && known.key) continue;
    await keyForSale(env, sess);
    made++;
  }
  return { made, checked };
}

async function adminApi(request, env) {
  const body = await request.json().catch(() => ({}));
  if (!(await passOK(env, body.pass, request))) return authFail(env, request);
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
      const days = Math.min(Math.max(Number(body.trialDays) || 0, 0), 365);
      const rec = { createdAt: Date.now(), hwid: null, revoked: false, note: String(body.note || "").slice(0, 200), source: "admin" };
      if (days > 0) rec.trialMs = Math.round(days * 86400000); // counts from when it's activated
      await putKey(env, key, rec);
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
    await putKey(env, k, rec);
    return json({ ok: true });
  }
  if (action === "extend" || action === "makeFull") {
    const k = normKey(body.key);
    const rec = await env.LICENSES.get(k, "json");
    if (!rec) return json({ ok: false, reason: "That key doesn't exist." });
    if (action === "makeFull") { delete rec.trialMs; delete rec.expiresAt; }
    else {
      const add = Math.min(Math.max(Number(body.days) || 7, 1), 365) * 86400000;
      if (rec.expiresAt) rec.expiresAt = Math.max(Date.now(), rec.expiresAt) + add;
      else rec.trialMs = (rec.trialMs || 0) + add;
    }
    await putKey(env, k, rec);
    return json({ ok: true, rec: keyMeta(rec) });
  }
  if (action === "note") {
    const k = normKey(body.key);
    const rec = await env.LICENSES.get(k, "json");
    if (!rec) return json({ ok: false, reason: "That key doesn't exist." });
    rec.note = String(body.note || "").slice(0, 200);
    await putKey(env, k, rec);
    return json({ ok: true });
  }
  if (action === "sales") return json({ ok: true, rows: await listSales(env) });
  if (action === "syncSales") return json({ ok: true, ...(await syncSales(env)) });

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
    // tidy up: keep this version and the one that was live until now; delete anything older
    const keep = new Set([version, live && live.version].filter(Boolean));
    const old = await env.LICENSES.list({ prefix: "file:app:" });
    for (const it of old.keys) if (!keep.has(it.name.slice(9))) await env.LICENSES.delete(it.name);
    const oldMeta = await env.LICENSES.list({ prefix: "filemeta:app:" });
    for (const it of oldMeta.keys) if (!keep.has(it.name.slice(13))) await env.LICENSES.delete(it.name);
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
      const t = String(text || "");
      const bytes = new TextEncoder().encode(t).length;
      if (bytes > 200 * 1024) return json({ ok: false, reason: clean + " is bigger than 200 KB." });
      total += bytes;
      bats[fn] = t;
    }
    if (Object.keys(bats).length > 30) return json({ ok: false, reason: "That's more than 30 .bat files." });
    if (total > 2 * 1024 * 1024) return json({ ok: false, reason: "Those .bat files are too big in total (2 MB max)." });
    const pack = { message: String(p.message || "").slice(0, 400), json: String(p.json || "").slice(0, 512 * 1024), bats, savedAt: Date.now() };
    await env.LICENSES.put("cfg:pack", JSON.stringify(pack));
    return json({ ok: true, pack });
  }

  return json({ ok: false, reason: "Unknown action" }, 400);
}

async function adminUpload(request, env, url) {
  if (request.method !== "POST") return json({ ok: false, reason: "POST only" }, 405);
  if (!(await passOK(env, request.headers.get("X-Admin-Pass"), request))) return authFail(env, request);
  needStorage(env);
  const version = String(url.searchParams.get("version") || "").trim().replace(/^v/i, "");
  if (!/^\d+(\.\d+){1,3}$/.test(version)) return json({ ok: false, reason: "Version must look like 1.9.0" });
  const size = Number(request.headers.get("Content-Length") || 0);
  if (!size || size < 1024 * 1024) return json({ ok: false, reason: "That file is too small to be the app." });
  if (size > 24 * 1024 * 1024) return json({ ok: false, reason: "That file is over 24 MB, which is too big to store." });
  // streamed straight into storage; the admin page already checked it's an .exe, and the app checks
  // the version written inside the file before installing it
  await env.LICENSES.put("file:app:" + version, request.body, { metadata: { size, uploadedAt: Date.now() } });
  await env.LICENSES.put("filemeta:app:" + version, JSON.stringify({ size, uploadedAt: Date.now() }));
  return json({ ok: true, size });
}

function simplePage(title, bodyHtml, origin) {
  return SIMPLE_HTML.split("{{TITLE}}").join(esc(title)).split("{{BODY}}").join(bodyHtml).split("{{ORIGIN}}").join(esc(origin || ""));
}

// ───────────────────────── pages (filled in by build_server.py) ─────────────────────────
const SITE_HTML = "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>AstroSpeed Tweaks — one-click PC optimizer for gamers<\/title>\n<meta name=\"description\" content=\"A free Windows optimizer for gamers. One-click Optimize, per-game tweaks, PC checkup and a live ping graph. Unlock the full version with a key.\">\n<link rel=\"preconnect\" href=\"https://fonts.googleapis.com\">\n<link rel=\"preconnect\" href=\"https://fonts.gstatic.com\" crossorigin>\n<link href=\"https://fonts.googleapis.com/css2?family=Manrope:wght@500;600;700;800&display=swap\" rel=\"stylesheet\">\n<style>\n  *{box-sizing:border-box;margin:0;padding:0}\n  :root{\n    --bg:#0B0B0C;--bg2:#141416;--card:#17171A;--card2:#1E1E22;\n    --ink:#F3F1EC;--ink2:#A6A8AE;--ink3:#6B6E74;--line:rgba(255,255,255,.08);\n    --accent:#FF6B3F;--accent2:#FFB27A;--accent3:#FF3D6E;--ok:#4FC27E;\n    --out:cubic-bezier(.16,1,.3,1);\n  }\n  html{scroll-behavior:smooth;-webkit-font-smoothing:antialiased}\n  body{font-family:\"Manrope\",system-ui,sans-serif;background:var(--bg);color:var(--ink);line-height:1.5;overflow-x:hidden}\n  a{color:inherit;text-decoration:none}\n  .wrap{max-width:1120px;margin:0 auto;padding:0 22px}\n  ::selection{background:rgba(255,107,63,.3)}\n\n  /* scroll progress */\n  #prog{position:fixed;top:0;left:0;height:3px;width:100%;transform:scaleX(0);transform-origin:0;background:linear-gradient(90deg,var(--accent3),var(--accent),var(--accent2));z-index:60}\n\n  .btn{position:relative;display:inline-flex;align-items:center;gap:9px;height:52px;padding:0 28px;border-radius:999px;font-weight:800;font-size:16px;border:0;cursor:pointer;white-space:nowrap;transition:transform .3s var(--out),box-shadow .3s var(--out);overflow:hidden;will-change:transform}\n  .btn svg{width:19px;height:19px}\n  .btn.primary{background:linear-gradient(120deg,var(--accent3),var(--accent) 55%,var(--accent2));color:#1b0a04;box-shadow:0 14px 44px -14px var(--accent)}\n  .btn.primary::after{content:\"\";position:absolute;inset:0;background:linear-gradient(110deg,transparent 30%,rgba(255,255,255,.45) 50%,transparent 70%);transform:translateX(-130%)}\n  .btn.primary:hover::after{animation:sheen .9s var(--out)}\n  @keyframes sheen{to{transform:translateX(130%)}}\n  .btn.ghost{background:rgba(255,255,255,.04);color:var(--ink);border:1.5px solid var(--line);backdrop-filter:blur(8px)}\n  .btn.ghost:hover{border-color:var(--ink2)}\n  .btn:hover{transform:translateY(-3px)}\n\n  /* header */\n  header{position:sticky;top:0;z-index:50;transition:background .4s,border-color .4s}\n  header.stuck{background:rgba(11,11,12,.78);backdrop-filter:blur(16px);border-bottom:1px solid var(--line)}\n  .nav{display:flex;align-items:center;gap:16px;height:70px}\n  .logo{display:flex;align-items:center;gap:11px;font-weight:800;font-size:18px;letter-spacing:-.02em}\n  .mark{width:38px;height:38px;border-radius:11px;background:linear-gradient(140deg,var(--accent3),var(--accent));display:grid;place-items:center;box-shadow:0 6px 20px -8px var(--accent)}\n  .mark svg{width:21px;height:21px}\n  .nav .sp{flex:1}\n  .nav a.lnk{color:var(--ink2);font-weight:600;font-size:15px;transition:color .25s}\n  .nav a.lnk:hover{color:var(--ink)}\n  @media(max-width:640px){.nav a.lnk{display:none}}\n\n  section{padding:96px 0;position:relative}\n\n  /* hero */\n  .hero{text-align:center;padding:110px 0 70px;min-height:92vh;display:flex;flex-direction:column;justify-content:center}\n  .aurora{position:absolute;inset:-20% -10% auto;height:820px;z-index:0;pointer-events:none;filter:blur(36px);opacity:.75}\n  .aurora i{position:absolute;border-radius:50%;mix-blend-mode:screen;animation:drift 18s ease-in-out infinite}\n  .aurora i:nth-child(1){width:540px;height:540px;left:14%;top:0;background:radial-gradient(circle,rgba(255,61,110,.5),transparent 60%)}\n  .aurora i:nth-child(2){width:620px;height:620px;right:10%;top:40px;background:radial-gradient(circle,rgba(255,107,63,.5),transparent 60%);animation-delay:-6s}\n  .aurora i:nth-child(3){width:420px;height:420px;left:40%;top:120px;background:radial-gradient(circle,rgba(255,178,122,.4),transparent 60%);animation-delay:-11s}\n  @keyframes drift{0%,100%{transform:translate(0,0) scale(1)}33%{transform:translate(40px,30px) scale(1.12)}66%{transform:translate(-30px,20px) scale(.94)}}\n  .grain{position:absolute;inset:0;z-index:1;pointer-events:none;opacity:.4;background-image:radial-gradient(rgba(255,255,255,.5) .5px,transparent .5px);background-size:4px 4px;mask-image:linear-gradient(180deg,transparent,#000 30%,#000 70%,transparent)}\n  .hero .wrap{position:relative;z-index:2}\n  .pill{display:inline-flex;align-items:center;gap:9px;padding:8px 16px;border:1px solid var(--line);border-radius:999px;font-size:13.5px;font-weight:700;color:var(--ink2);background:rgba(255,255,255,.03);backdrop-filter:blur(8px);margin-bottom:28px}\n  .pill b{color:var(--accent)}\n  .pdot{width:7px;height:7px;border-radius:50%;background:var(--ok);box-shadow:0 0 0 0 rgba(79,194,126,.6);animation:pulse 2s infinite}\n  @keyframes pulse{70%{box-shadow:0 0 0 7px rgba(79,194,126,0)}}\n  h1{font-size:clamp(44px,8vw,86px);line-height:.98;letter-spacing:-.045em;font-weight:800;max-width:16ch;margin:0 auto 24px}\n  h1 .w{display:inline-block;opacity:0;transform:translateY(90%) rotate(4deg);animation:rise .9s var(--out) forwards}\n  h1 .g{background:linear-gradient(110deg,var(--accent3),var(--accent),var(--accent2),var(--accent));background-size:260% 100%;-webkit-background-clip:text;background-clip:text;color:transparent;animation:rise .9s var(--out) forwards,flow 6s linear infinite}\n  @keyframes rise{to{opacity:1;transform:none}}\n  @keyframes flow{to{background-position:260% 0}}\n  .sub{font-size:clamp(17px,2.3vw,22px);color:var(--ink2);max-width:60ch;margin:0 auto 38px;opacity:0;animation:fade 1s .7s var(--out) forwards}\n  @keyframes fade{to{opacity:1}}\n  .cta{display:flex;gap:14px;justify-content:center;flex-wrap:wrap;opacity:0;animation:fade 1s .9s var(--out) forwards}\n  .mini{margin-top:20px;font-size:13.5px;color:var(--ink3);opacity:0;animation:fade 1s 1.1s var(--out) forwards}\n\n  /* floating app card */\n  .stage{margin:66px auto 0;max-width:960px;opacity:0;transform:translateY(40px);animation:fade 1.2s 1.1s var(--out) forwards,floatUp 1.2s 1.1s var(--out) forwards;perspective:1600px}\n  @keyframes floatUp{to{transform:none}}\n  .glass{border-radius:22px;border:1px solid var(--line);background:linear-gradient(180deg,rgba(30,30,34,.9),rgba(20,20,22,.9));padding:12px;box-shadow:0 60px 140px -50px rgba(0,0,0,.95),0 0 0 1px rgba(255,255,255,.03);transform-style:preserve-3d;transition:transform .2s ease-out}\n  .scorecard{display:grid;grid-template-columns:168px 1fr;gap:30px;align-items:center;padding:34px;background:var(--bg2);border-radius:14px}\n  @media(max-width:600px){.scorecard{grid-template-columns:1fr;text-align:center;gap:18px;padding:26px}}\n  .ring{width:168px;height:168px;position:relative;margin:0 auto}\n  .ring svg{transform:rotate(-90deg)}\n  .ring .trk{stroke:rgba(255,255,255,.08)}\n  .ring .val{stroke:url(#rg);stroke-linecap:round;filter:drop-shadow(0 6px 16px rgba(255,107,63,.4))}\n  .ring .n{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center}\n  .ring .n b{font-size:52px;letter-spacing:-.04em;font-weight:800;line-height:1}\n  .ring .n span{font-size:11px;color:var(--ink3);margin-top:4px;letter-spacing:.1em}\n  .scoretext h3{font-size:25px;font-weight:800;letter-spacing:-.02em;margin-bottom:8px}\n  .scoretext p{color:var(--ink2);font-size:15px;max-width:46ch;margin:0 auto}\n  .chips{display:flex;gap:8px;flex-wrap:wrap;margin-top:16px}\n  .chip{font-size:12px;font-weight:700;padding:6px 11px;border-radius:999px;background:rgba(255,255,255,.05);color:var(--ink2);display:flex;align-items:center;gap:6px}\n  .chip.good{color:var(--ok)}.chip.bad{color:var(--accent)}\n\n  /* game marquee */\n  .marq{overflow:hidden;padding:30px 0;border-top:1px solid var(--line);border-bottom:1px solid var(--line);mask-image:linear-gradient(90deg,transparent,#000 12%,#000 88%,transparent)}\n  .marq .row{display:flex;gap:56px;width:max-content;animation:scrollx 26s linear infinite}\n  .marq span{font-size:21px;font-weight:800;color:var(--ink3);letter-spacing:-.02em;white-space:nowrap;transition:color .3s}\n  .marq:hover span{color:var(--ink2)}\n  @keyframes scrollx{to{transform:translateX(-50%)}}\n\n  .eyebrow{text-align:center;font-size:13px;font-weight:800;letter-spacing:.16em;text-transform:uppercase;color:var(--accent);margin-bottom:14px}\n  h2{text-align:center;font-size:clamp(32px,5vw,50px);letter-spacing:-.04em;font-weight:800;margin-bottom:16px;line-height:1.05}\n  .lead{text-align:center;color:var(--ink2);font-size:17px;max-width:56ch;margin:0 auto 56px}\n\n  /* stats */\n  .stats{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;max-width:760px;margin:0 auto}\n  @media(max-width:640px){.stats{grid-template-columns:1fr}}\n  .stat{text-align:center;padding:26px;border:1px solid var(--line);border-radius:18px;background:var(--card)}\n  .stat b{display:block;font-size:46px;font-weight:800;letter-spacing:-.03em;background:linear-gradient(120deg,var(--accent),var(--accent2));-webkit-background-clip:text;background-clip:text;color:transparent}\n  .stat span{color:var(--ink2);font-size:14.5px;font-weight:600}\n\n  /* features */\n  .grid{display:grid;grid-template-columns:repeat(3,1fr);gap:18px}\n  @media(max-width:820px){.grid{grid-template-columns:1fr}}\n  .feat{position:relative;background:var(--card);border:1px solid var(--line);border-radius:20px;padding:28px;transition:transform .5s var(--out),border-color .4s,background .4s;transform-style:preserve-3d}\n  .feat:hover{border-color:rgba(255,107,63,.45);background:var(--card2)}\n  .feat .ic{width:48px;height:48px;border-radius:13px;background:linear-gradient(140deg,rgba(255,61,110,.18),rgba(255,107,63,.14));display:grid;place-items:center;margin-bottom:18px}\n  .feat .ic svg{width:24px;height:24px;color:var(--accent)}\n  .feat h3{font-size:18.5px;font-weight:800;margin-bottom:8px;letter-spacing:-.01em}\n  .feat p{color:var(--ink2);font-size:14.5px}\n\n  /* pricing */\n  .price-wrap{display:grid;grid-template-columns:1fr 1fr;gap:20px;max-width:780px;margin:0 auto}\n  @media(max-width:720px){.price-wrap{grid-template-columns:1fr}}\n  .plan{position:relative;background:var(--card);border:1px solid var(--line);border-radius:24px;padding:34px;overflow:hidden}\n  .plan.pro{border:1.5px solid transparent;background:linear-gradient(var(--card),var(--card)) padding-box,linear-gradient(140deg,var(--accent3),var(--accent),var(--accent2)) border-box}\n  .plan.pro::before{content:\"\";position:absolute;top:-60px;right:-60px;width:200px;height:200px;background:radial-gradient(circle,rgba(255,107,63,.25),transparent 65%);pointer-events:none}\n  .plan .tag{position:absolute;top:22px;right:22px;background:linear-gradient(120deg,var(--accent3),var(--accent));color:#1b0a04;font-size:11.5px;font-weight:800;padding:5px 12px;border-radius:999px;letter-spacing:.03em}\n  .plan h3{font-size:14px;font-weight:800;letter-spacing:.06em;text-transform:uppercase;color:var(--ink2);margin-bottom:14px}\n  .plan .amt{font-size:52px;font-weight:800;letter-spacing:-.03em;margin-bottom:4px;line-height:1}\n  .plan .amt small{font-size:16px;color:var(--ink3);font-weight:600;letter-spacing:0}\n  .plan .note{color:var(--ink3);font-size:13.5px;margin-bottom:24px}\n  .plan ul{list-style:none;display:flex;flex-direction:column;gap:12px;margin-bottom:28px}\n  .plan li{display:flex;gap:10px;align-items:flex-start;font-size:14.5px}\n  .plan li svg{width:18px;height:18px;flex:none;margin-top:1px;color:var(--ok)}\n  .plan .btn{width:100%;justify-content:center;height:50px}\n\n  /* faq */\n  .faq{max-width:760px;margin:40px auto 0}\n  details{border:1px solid var(--line);border-radius:14px;padding:4px 20px;margin-bottom:12px;background:var(--card);transition:border-color .3s}\n  details[open]{border-color:rgba(255,107,63,.35)}\n  summary{font-weight:700;font-size:16.5px;cursor:pointer;list-style:none;display:flex;justify-content:space-between;align-items:center;gap:12px;padding:16px 0}\n  summary::-webkit-details-marker{display:none}\n  summary .plus{color:var(--accent);font-size:24px;transition:transform .3s var(--out);flex:none}\n  details[open] summary .plus{transform:rotate(135deg)}\n  details p{color:var(--ink2);padding:0 0 18px;font-size:15px}\n\n  /* final cta */\n  .final{text-align:center;border-radius:28px;margin:0 22px;padding:70px 24px;position:relative;overflow:hidden;background:radial-gradient(ellipse at top,rgba(255,107,63,.14),transparent 60%),var(--bg2);border:1px solid var(--line)}\n  footer{border-top:1px solid var(--line);padding:42px 0;text-align:center;color:var(--ink3);font-size:14px;margin-top:60px}\n\n  .reveal{opacity:0;transform:translateY(30px);transition:opacity .8s var(--out),transform .8s var(--out)}\n  .reveal.in{opacity:1;transform:none}\n  .stg>*{transition-delay:calc(var(--i,0)*80ms)}\n  @media(prefers-reduced-motion:reduce){*{animation:none!important;transition:none!important}.reveal{opacity:1;transform:none}h1 .w,h1 .g,.sub,.cta,.mini,.stage{opacity:1;transform:none}}\n<\/style>\n<\/head>\n<body>\n<div id=\"prog\"><\/div>\n<header><div class=\"wrap nav\">\n  <div class=\"logo\"><span class=\"mark\"><svg viewBox=\"0 0 24 24\"><path d=\"M21 3 3 10.5l7.5 2.5L13 21z\" fill=\"none\" stroke=\"#fff\" stroke-width=\"1.7\" stroke-linejoin=\"round\"/><path d=\"m10.5 13 4-4\" stroke=\"#fff\" stroke-width=\"1.7\"/><\/svg><\/span>AstroSpeed<\/div>\n  <div class=\"sp\"><\/div>\n  <a class=\"lnk\" href=\"#features\">Features<\/a>\n  <a class=\"lnk\" href=\"#pricing\">Pricing<\/a>\n  <a class=\"lnk\" href=\"#faq\">FAQ<\/a>\n  <a class=\"btn ghost\" style=\"height:42px;font-size:14.5px;padding:0 18px\" id=\"dlTop\" href=\"/download\">Download free<\/a>\n<\/div><\/header>\n\n<section class=\"hero\">\n  <div class=\"aurora\"><i><\/i><i><\/i><i><\/i><\/div>\n  <div class=\"grain\"><\/div>\n  <div class=\"wrap\">\n    <div class=\"pill\"><span class=\"pdot\"><\/span><b>Free<\/b> to download · unlock everything with a key<\/div>\n    <h1 id=\"headline\"><\/h1>\n    <p class=\"sub\">AstroSpeed finds what's holding your games back and fixes it in one click — safely, and every change is reversible. Built for Fortnite, Valorant, CS2, Apex and more.<\/p>\n    <div class=\"cta\">\n      <a class=\"btn primary magnet\" href=\"#download\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M12 3v12m0 0 4-4m-4 4-4-4M4 19h16\"/><\/svg>Download free<\/a>\n      <a class=\"btn ghost magnet\" href=\"#pricing\">See what a key unlocks<\/a>\n    <\/div>\n    <div class=\"mini\">Windows 10 & 11 · No sensitivity or in-game settings ever touched<\/div>\n\n    <div class=\"stage\"><div class=\"glass\" id=\"tilt\"><div class=\"scorecard\">\n      <div class=\"ring\"><svg width=\"168\" height=\"168\" viewBox=\"0 0 168 168\"><circle class=\"trk\" cx=\"84\" cy=\"84\" r=\"72\" fill=\"none\" stroke-width=\"12\"/><circle class=\"val\" id=\"ring\" cx=\"84\" cy=\"84\" r=\"72\" fill=\"none\" stroke-width=\"12\" stroke-dasharray=\"452\" stroke-dashoffset=\"452\"/><defs><linearGradient id=\"rg\" x1=\"0\" y1=\"0\" x2=\"1\" y2=\"1\"><stop offset=\"0\" stop-color=\"#FF3D6E\"/><stop offset=\"1\" stop-color=\"#FFB27A\"/><\/linearGradient><\/defs><\/svg><div class=\"n\"><b id=\"scoreN\">0<\/b><span>PC SCORE<\/span><\/div><\/div>\n      <div class=\"scoretext\"><h3>Know exactly what's slowing you down<\/h3><p>The PC Checkup scans your refresh rate, power plan, memory, drivers and more — then fixes the fixable ones in a click.<\/p>\n        <div class=\"chips\"><span class=\"chip bad\">⚠ 165Hz screen at 60Hz<\/span><span class=\"chip bad\">⚠ Balanced power plan<\/span><span class=\"chip good\">✓ Game Mode on<\/span><\/div>\n      <\/div>\n    <\/div><\/div><\/div>\n  <\/div>\n<\/section>\n\n<div class=\"marq\"><div class=\"row\" id=\"marq\"><\/div><\/div>\n\n<section id=\"features\"><div class=\"wrap\">\n  <div class=\"eyebrow reveal\">What it does<\/div>\n  <h2 class=\"reveal\">Everything in one place<\/h2>\n  <p class=\"lead reveal\">No scattered guides or risky registry edits. AstroSpeed does the work and keeps a backup of every change.<\/p>\n  <div class=\"grid stg\">\n    <div class=\"feat reveal\" style=\"--i:0\"><div class=\"ic\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M21 3 3 10.5l7.5 2.5L13 21z\"/><\/svg><\/div><h3>One-click Optimize<\/h3><p>Pick Recommended or Competitive and AstroSpeed applies dozens of safe tweaks at once, after making a restore point.<\/p><\/div>\n    <div class=\"feat reveal\" style=\"--i:1\"><div class=\"ic\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M2.5 12h4l2.5-6 4.5 12 2.5-6h5.5\"/><\/svg><\/div><h3>PC Checkup<\/h3><p>A full scan with a score out of 100 and one-click fixes for the things actually costing you frames.<\/p><\/div>\n    <div class=\"feat reveal\" style=\"--i:2\"><div class=\"ic\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M7 8h10a4.5 4.5 0 0 1 4.4 5.5l-.8 3.2A3 3 0 0 1 14.8 19L13 17h-2l-1.8 2a3 3 0 0 1-5.8-2.3l-.8-3.2A4.5 4.5 0 0 1 7 8Z\"/><\/svg><\/div><h3>Per-game tweaks<\/h3><p>GPU choice, high priority and network tagging for each game you have installed — nothing that touches anti-cheat.<\/p><\/div>\n    <div class=\"feat reveal\" style=\"--i:0\"><div class=\"ic\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M4 20v-3M9 20v-7M14 20V9M19 20V4\"/><\/svg><\/div><h3>Live ping graph<\/h3><p>Watch your ping to every game region second by second, so you can see spikes and lag before a match.<\/p><\/div>\n    <div class=\"feat reveal\" style=\"--i:1\"><div class=\"ic\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M14.5 4.5c2.8-1.6 5.4-1.8 6.5-1.5.3 1.1.1 3.7-1.5 6.5M15 9l-4.5 4.5M9 15l-5 2 1-4\"/><\/svg><\/div><h3>Game Booster<\/h3><p>Free memory, tighten the system timer and close background apps right before you queue — or on a hotkey.<\/p><\/div>\n    <div class=\"feat reveal\" style=\"--i:2\"><div class=\"ic\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M9 14 4 9l5-5M4 9h10.5a5.5 5.5 0 0 1 0 11H11\"/><\/svg><\/div><h3>Fully reversible<\/h3><p>Every change is backed up first. Undo one tweak, a whole section, or everything — any time.<\/p><\/div>\n  <\/div>\n<\/div><\/section>\n\n<section style=\"padding-top:0\"><div class=\"wrap\">\n  <div class=\"stats stg\">\n    <div class=\"stat reveal\" style=\"--i:0\"><b data-to=\"150\">0<\/b><span>reversible tweaks<\/span><\/div>\n    <div class=\"stat reveal\" style=\"--i:1\"><b data-to=\"12\">0<\/b><span>games with their own tuning<\/span><\/div>\n    <div class=\"stat reveal\" style=\"--i:2\"><b data-to=\"100\">0<\/b><span>point PC health score<\/span><\/div>\n  <\/div>\n<\/div><\/section>\n\n<section id=\"pricing\" style=\"background:var(--bg2)\"><div class=\"wrap\">\n  <div class=\"eyebrow reveal\">Pricing<\/div>\n  <h2 class=\"reveal\">Start free. Unlock when you're ready.<\/h2>\n  <p class=\"lead reveal\">One key, one PC, forever. No subscription.<\/p>\n  <div class=\"price-wrap\">\n    <div class=\"plan reveal\">\n      <h3>Free<\/h3>\n      <div class=\"amt\">$0<\/div>\n      <div class=\"note\">No account, no sign-up<\/div>\n      <ul>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>Dashboard & PC Checkup<\/li>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>Game Booster & System Cleaner<\/li>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>Safe recommended tweaks<\/li>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>Ping test<\/li>\n      <\/ul>\n      <a class=\"btn ghost\" href=\"#download\">Download free<\/a>\n    <\/div>\n    <div class=\"plan pro reveal\">\n      <span class=\"tag\">FULL VERSION<\/span>\n      <h3>With a key<\/h3>\n      <div class=\"amt\" style=\"white-space:nowrap\"><span id=\"price\">$5<\/span> <small><span id=\"priceCur\">AUD<\/span> one-time<\/small><\/div>\n      <div class=\"note\">Locked to your PC · never expires<\/div>\n      <ul>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>Everything in Free, plus:<\/li>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>Full one-click Optimize (Competitive)<\/li>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>All per-game tweaks<\/li>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>Network, GPU & BIOS tweaks<\/li>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>Auto-boost & the boost hotkey<\/li>\n        <li><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.4\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg>New live tweaks pushed to the app<\/li>\n      <\/ul>\n      <a class=\"btn primary magnet\" id=\"buyBtn\" href=\"/buy\">Get a key<\/a>\n    <\/div>\n  <\/div>\n  <p style=\"text-align:center;color:var(--ink3);font-size:13.5px;margin-top:26px\">Secure checkout by Stripe. Your key appears on screen the moment you pay. Then open AstroSpeed → <b>Free version · Unlock<\/b> → paste it in.<\/p>\n<\/div><\/section>\n\n<section id=\"faq\"><div class=\"wrap\">\n  <div class=\"eyebrow reveal\">Questions<\/div>\n  <h2 class=\"reveal\">Good to know<\/h2>\n  <div class=\"faq\">\n    <details class=\"reveal\"><summary>Is it safe? <span class=\"plus\">+<\/span><\/summary><p>Yes. AstroSpeed backs up the exact value of every setting before it changes it, offers to make a Windows restore point, and lets you undo any change. It never touches your mouse sensitivity, your in-game settings, or anti-cheat.<\/p><\/details>\n    <details class=\"reveal\"><summary>Does a key work on more than one PC? <span class=\"plus\">+<\/span><\/summary><p>No. Each key locks to the first PC that activates it, so it can't be shared. If you get a new PC, <span class=\"support\">get in touch<\/span> and your key can be moved over.<\/p><\/details>\n    <details class=\"reveal\"><summary>I lost my key. What now? <span class=\"plus\">+<\/span><\/summary><p>Go back to the page you saw after paying (the link is in your browser history) or <span class=\"support\">contact support<\/span> with the email you paid with, and you'll get it again.<\/p><\/details>\n    <details class=\"reveal\"><summary>Is it a subscription? <span class=\"plus\">+<\/span><\/summary><p>No. A key is a one-time purchase and never expires.<\/p><\/details>\n    <details class=\"reveal\"><summary>Will it get me banned? <span class=\"plus\">+<\/span><\/summary><p>No. AstroSpeed only changes Windows settings. It never modifies game files, memory or anything anti-cheat looks at.<\/p><\/details>\n    <details class=\"reveal\"><summary>What if I don't like it? <span class=\"plus\">+<\/span><\/summary><p>The free version lets you try the booster, checkup and safe tweaks before you ever buy a key, so you know what you're getting.<\/p><\/details>\n  <\/div>\n<\/div><\/section>\n\n<section id=\"download\"><div class=\"wrap\">\n  <div class=\"final reveal\">\n    <div class=\"eyebrow\">Download<\/div>\n    <h2>Ready for more frames?<\/h2>\n    <p class=\"lead\" style=\"margin-bottom:34px\">Free to try, no account needed. Works on Windows 10 and 11.<\/p>\n    <a class=\"btn primary magnet\" id=\"dlBtn\" href=\"/download\" style=\"margin:0 auto\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M12 3v12m0 0 4-4m-4 4-4-4M4 19h16\"/><\/svg>Download for Windows<\/a>\n    <p style=\"color:var(--ink3);font-size:13.5px;margin-top:18px\">First run may show \"Windows protected your PC\" — click <b>More info → Run anyway<\/b>. Normal for new indie apps.<\/p>\n  <\/div>\n<\/div><\/section>\n\n<footer><div class=\"wrap\">\n  AstroSpeed Tweaks · A Windows optimizer for gamers. Not affiliated with Epic Games, Riot, Valve or any game shown.<br>\n  Every change is reversible. Use at your own discretion.\n<\/div><\/footer>\n\n<script>\n  // Price, buy button and support email come from your admin page (Settings tab) — nothing to edit here.\n  (function () {\n    var buy = document.getElementById('buyBtn');\n    fetch('/config.json').then(function (r) { return r.json(); }).then(function (c) {\n      if (c.priceShort) document.getElementById('price').textContent = c.priceShort;\n      if (c.currencyCode) document.getElementById('priceCur').textContent = c.currencyCode;\n      if (!c.buyEnabled) { buy.textContent = 'Coming soon'; buy.style.opacity = '.6'; buy.style.pointerEvents = 'none'; buy.removeAttribute('href'); }\n      var sup = document.querySelectorAll('.support');\n      for (var i = 0; i < sup.length; i++) {\n        if (c.supportEmail) { var a = document.createElement('a'); a.href = 'mailto:' + c.supportEmail; a.textContent = sup[i].textContent; a.style.color = 'var(--accent)'; sup[i].replaceWith(a); }\n      }\n    }).catch(function () {});\n  })();\n\n  var reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;\n\n  // kinetic headline\n  (function(){\n    var words = [[\"Your\",0],[\"PC,\",0],[\"tuned\",0],[\"for\",0],[\"more\",1],[\"frames.\",1]];\n    document.getElementById('headline').innerHTML = words.map(function(w,i){\n      return '<span class=\"'+(w[1]?'w g':'w')+'\" style=\"animation-delay:'+(0.15+i*0.08)+'s\">'+w[0]+'<\/span>';\n    }).join(' ');\n  })();\n\n  // scroll progress + sticky header\n  var prog = document.getElementById('prog'), hdr = document.querySelector('header');\n  addEventListener('scroll', function(){\n    var h = document.documentElement, max = h.scrollHeight - h.clientHeight;\n    prog.style.transform = 'scaleX('+(max>0?h.scrollTop/max:0)+')';\n    hdr.classList.toggle('stuck', h.scrollTop > 20);\n  }, {passive:true});\n\n  // reveal on scroll\n  var io = new IntersectionObserver(function(es){es.forEach(function(e){if(e.isIntersecting){e.target.classList.add('in');io.unobserve(e.target);}})},{threshold:.14});\n  document.querySelectorAll('.reveal').forEach(function(el){io.observe(el);});\n\n  // count-up helper\n  function countUp(el, to, dur){ if(reduce){el.textContent=to;return;} var t0=performance.now(); (function step(now){var k=Math.min(1,(now-t0)/dur);var e=1-Math.pow(1-k,4);el.textContent=Math.round(to*e);if(k<1)requestAnimationFrame(step);})(performance.now()); }\n\n  // score ring + number when hero in view\n  (function(){\n    var ring=document.getElementById('ring'), n=document.getElementById('scoreN'), C=452, score=78;\n    setTimeout(function(){ if(!reduce){ring.style.transition='stroke-dashoffset 1.8s cubic-bezier(.16,1,.3,1)';} ring.style.strokeDashoffset = C*(1-score/100); countUp(n,score,1800); }, 1400);\n  })();\n\n  // stat counters\n  var sio = new IntersectionObserver(function(es){es.forEach(function(e){if(e.isIntersecting){countUp(e.target,+e.target.dataset.to,1400);sio.unobserve(e.target);}})},{threshold:.6});\n  document.querySelectorAll('[data-to]').forEach(function(el){sio.observe(el);});\n\n  // game marquee\n  (function(){\n    var games=[\"Fortnite\",\"Valorant\",\"Counter-Strike 2\",\"Apex Legends\",\"Rust\",\"Sea of Thieves\",\"Roblox\",\"Overwatch 2\",\"Marvel Rivals\",\"Rocket League\",\"League of Legends\",\"GTA V\"];\n    var two=games.concat(games);\n    document.getElementById('marq').innerHTML = two.map(function(g){return '<span>'+g+' ·<\/span>';}).join('');\n  })();\n\n  // magnetic buttons + card tilt (skipped if reduced motion or touch)\n  if(!reduce && matchMedia('(pointer:fine)').matches){\n    document.querySelectorAll('.magnet').forEach(function(b){\n      b.addEventListener('mousemove', function(e){var r=b.getBoundingClientRect();b.style.transform='translate('+((e.clientX-r.left-r.width/2)*0.25)+'px,'+((e.clientY-r.top-r.height/2)*0.4-3)+'px)';});\n      b.addEventListener('mouseleave', function(){b.style.transform='';});\n    });\n    var tilt=document.getElementById('tilt');\n    if(tilt){ tilt.addEventListener('mousemove',function(e){var r=tilt.getBoundingClientRect();var rx=((e.clientY-r.top)/r.height-.5)*-6;var ry=((e.clientX-r.left)/r.width-.5)*8;tilt.style.transform='rotateX('+rx+'deg) rotateY('+ry+'deg)';});\n      tilt.addEventListener('mouseleave',function(){tilt.style.transform='';}); }\n  }\n<\/script>\n<\/body>\n<\/html>\n";
const ADMIN_HTML = "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<meta name=\"robots\" content=\"noindex\">\n<title>AstroSpeed control panel<\/title>\n<style>\n*{box-sizing:border-box;margin:0;padding:0}\n:root{--bg:#0B0B0C;--bg2:#121214;--card:#17171A;--card2:#1E1E22;--ink:#F3F1EC;--ink2:#A6A8AE;--ink3:#6B6E74;--line:rgba(255,255,255,.1);--accent:#FF6B3F;--ok:#4FC27E;--warn:#F2B347;--bad:#FF5A6E}\nhtml,body{background:var(--bg);color:var(--ink);font-family:\"Segoe UI\",system-ui,sans-serif;font-size:14px}\nbody{padding:24px 16px 60px}\n.wrap{max-width:1120px;margin:0 auto}\na{color:var(--accent)}\n.hd{display:flex;align-items:center;gap:12px;margin-bottom:20px}\n.mk{width:40px;height:40px;border-radius:11px;background:linear-gradient(140deg,#FF3D6E,#FF6B3F);display:grid;place-items:center;flex:none}\n.mk svg{width:22px;height:22px}\n.hd h1{font-size:20px;font-weight:800;letter-spacing:-.02em}\n.hd .sub{color:var(--ink3);font-size:12.5px}\n.hd .sp{flex:1}\n.card{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:20px}\ninput,button,select,textarea{font:inherit;color:inherit}\ninput,select,textarea{background:var(--card2);border:1px solid var(--line);border-radius:10px;padding:10px 12px;width:100%;color:var(--ink)}\ntextarea{min-height:110px;font-family:ui-monospace,Consolas,monospace;font-size:12.5px;resize:vertical}\ninput:focus,select:focus,textarea:focus{outline:none;border-color:var(--accent)}\nbutton{background:var(--accent);border:0;border-radius:10px;color:#1b0a04;font-weight:800;padding:10px 16px;cursor:pointer;white-space:nowrap}\nbutton:disabled{opacity:.5;cursor:default}\nbutton.g{background:var(--card2);color:var(--ink);border:1px solid var(--line);font-weight:700}\nbutton.sm{padding:6px 10px;font-size:12.5px;border-radius:8px}\nbutton.dz{background:transparent;color:var(--bad);border:1px solid rgba(255,90,110,.45);font-weight:700}\nlabel.f{display:block;font-size:12.5px;color:var(--ink2);font-weight:600;margin:14px 0 6px}\n.muted{color:var(--ink3)}\n.small{font-size:12.5px}\n.hide{display:none!important}\n.checks{display:flex;flex-wrap:wrap;gap:8px;margin-bottom:18px}\n.chk{display:flex;align-items:center;gap:7px;background:var(--card);border:1px solid var(--line);border-radius:999px;padding:7px 12px;font-size:12.5px;font-weight:600}\n.chk i{width:9px;height:9px;border-radius:50%;background:var(--ink3);flex:none}\n.chk.ok i{background:var(--ok)}.chk.warn i{background:var(--warn)}.chk.bad i{background:var(--bad)}\n.login{max-width:400px;margin:6vh auto 0}\n.login h2{font-size:18px;margin-bottom:6px}\n.status{min-height:20px;margin-top:12px;font-size:13px;font-weight:600}\n.status.err{color:var(--bad)}.status.ok{color:var(--ok)}.status.info{color:var(--ink2)}\n.tabs{display:flex;gap:6px;border-bottom:1px solid var(--line);margin-bottom:18px;overflow-x:auto}\n.tab{background:none;border:0;border-bottom:2px solid transparent;border-radius:0;color:var(--ink2);padding:10px 14px;font-weight:700}\n.tab.on{color:var(--ink);border-bottom-color:var(--accent)}\n.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:16px}\n.stat{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:14px 16px}\n.stat b{display:block;font-size:26px;font-weight:800;letter-spacing:-.02em}\n.stat span{font-size:12px;color:var(--ink3);font-weight:600}\n.stat.a b{color:var(--ok)}.stat.u b{color:var(--warn)}.stat.r b{color:var(--bad)}\n.bar{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-bottom:14px}\n.bar .grow{flex:1;min-width:170px;width:auto}\n.made{background:linear-gradient(180deg,rgba(255,107,63,.13),var(--card));border:1px solid rgba(255,107,63,.45);border-radius:14px;padding:16px 18px;margin-bottom:14px}\n.made .keys{font-family:ui-monospace,Consolas,monospace;font-size:15px;font-weight:700;line-height:1.9;white-space:pre-wrap;word-break:break-all;margin:8px 0 12px}\n.tbl{background:var(--card);border:1px solid var(--line);border-radius:14px;overflow-x:auto}\ntable{width:100%;border-collapse:collapse;min-width:640px}\nth{text-align:left;padding:11px 12px;color:var(--ink3);font-weight:700;font-size:11px;letter-spacing:.05em;text-transform:uppercase;border-bottom:1px solid var(--line)}\ntd{padding:11px 12px;border-bottom:1px solid var(--line);vertical-align:middle;font-size:13px}\ntr:last-child td{border-bottom:0}\n.k{font-family:ui-monospace,Consolas,monospace;font-weight:700;white-space:nowrap}\n.hw{font-family:ui-monospace,Consolas,monospace;font-size:11.5px;color:var(--ink2)}\n.tag{font-size:11px;font-weight:800;padding:3px 9px;border-radius:999px;white-space:nowrap}\n.t-trial{background:rgba(255,107,63,.15);color:var(--accent)}\n.t-unused{background:rgba(242,179,71,.16);color:var(--warn)}.t-active{background:rgba(79,194,126,.16);color:var(--ok)}.t-revoked{background:rgba(255,90,110,.16);color:var(--bad)}\n.acts{display:flex;gap:6px;justify-content:flex-end;flex-wrap:wrap}\n.cp{background:none;border:0;color:var(--ink3);padding:2px 5px;font-weight:400}\n.cp:hover{color:var(--accent)}\n.empty{padding:34px;text-align:center;color:var(--ink3)}\n.grid2{display:grid;grid-template-columns:1fr 1fr;gap:16px}\n.cur{display:flex;gap:18px;flex-wrap:wrap;align-items:baseline}\n.cur b{font-size:26px;font-weight:800}\n.prog{height:9px;border-radius:999px;background:var(--card2);overflow:hidden;margin-top:12px}\n.prog i{display:block;height:100%;width:0;background:var(--accent);transition:width .2s}\n.batrow{display:flex;align-items:center;gap:10px;padding:9px 0;border-bottom:1px solid var(--line)}\n.batrow:last-child{border-bottom:0}\n.batrow .n{flex:1;font-family:ui-monospace,Consolas,monospace;font-weight:700;font-size:13px;word-break:break-all}\n.links div{display:flex;gap:10px;align-items:center;padding:7px 0;border-bottom:1px solid var(--line);flex-wrap:wrap}\n.links div:last-child{border-bottom:0}\n.links span{width:110px;color:var(--ink3);font-size:12.5px;font-weight:600}\n.links code{font-family:ui-monospace,Consolas,monospace;font-size:12.5px;word-break:break-all;flex:1}\n.toast{position:fixed;left:50%;bottom:24px;transform:translateX(-50%);background:var(--card2);border:1px solid var(--line);border-radius:12px;padding:11px 16px;font-weight:600;box-shadow:0 20px 50px -20px #000;z-index:9}\n@media(max-width:760px){.stats{grid-template-columns:1fr 1fr}.grid2{grid-template-columns:1fr}}\n<\/style>\n<\/head>\n<body>\n<div class=\"wrap\">\n  <div class=\"hd\">\n    <span class=\"mk\"><svg viewBox=\"0 0 24 24\"><path d=\"M21 3 3 10.5l7.5 2.5L13 21z\" fill=\"none\" stroke=\"#fff\" stroke-width=\"1.7\" stroke-linejoin=\"round\"/><path d=\"m10.5 13 4-4\" stroke=\"#fff\" stroke-width=\"1.7\"/><\/svg><\/span>\n    <div><h1>AstroSpeed control panel<\/h1><div class=\"sub\">Keys, sales, releases and settings<\/div><\/div>\n    <div class=\"sp\"><\/div>\n    <button class=\"g sm hide\" id=\"signOut\" type=\"button\">Sign out<\/button>\n  <\/div>\n\n  <div class=\"checks\" id=\"checks\"><div class=\"chk\"><i><\/i>Checking setup…<\/div><\/div>\n\n  <form id=\"loginForm\" class=\"login card\" autocomplete=\"on\">\n    <h2>Sign in<\/h2>\n    <p class=\"muted small\">Enter the ADMIN_PASS you set in the worker's Settings.<\/p>\n    <label class=\"f\" for=\"pw\">Password<\/label>\n    <input id=\"pw\" type=\"password\" autocomplete=\"current-password\" autofocus>\n    <button id=\"go\" type=\"submit\" style=\"width:100%;margin-top:14px\">Sign in<\/button>\n    <div class=\"status info\" id=\"loginStatus\"><\/div>\n  <\/form>\n\n  <div id=\"app\" class=\"hide\">\n    <div class=\"tabs\" id=\"tabs\">\n      <button class=\"tab on\" data-tab=\"keys\" type=\"button\">Keys<\/button>\n      <button class=\"tab\" data-tab=\"sales\" type=\"button\">Sales<\/button>\n      <button class=\"tab\" data-tab=\"release\" type=\"button\">Release update<\/button>\n      <button class=\"tab\" data-tab=\"pack\" type=\"button\">Live tweaks<\/button>\n      <button class=\"tab\" data-tab=\"settings\" type=\"button\">Settings<\/button>\n    <\/div>\n\n    <section id=\"tab-keys\">\n      <div class=\"stats\" id=\"keyStats\"><\/div>\n      <div id=\"made\"><\/div>\n      <div class=\"bar\">\n        <input id=\"count\" type=\"number\" value=\"1\" min=\"1\" max=\"50\" style=\"width:80px\" title=\"How many keys\">\n        <select id=\"kind\" style=\"width:auto\"><option value=\"0\">Full key (forever)<\/option><option value=\"1\">Trial · 1 day<\/option><option value=\"3\">Trial · 3 days<\/option><option value=\"7\">Trial · 7 days<\/option><option value=\"14\">Trial · 14 days<\/option><option value=\"30\">Trial · 30 days<\/option><\/select>\n        <input id=\"note\" class=\"grow\" placeholder=\"Note (optional), e.g. who it's for\">\n        <button id=\"gen\" type=\"button\">+ Make keys<\/button>\n      <\/div>\n      <div class=\"bar\">\n        <input id=\"search\" class=\"grow\" placeholder=\"Search key, PC or note\">\n        <button class=\"g\" id=\"refreshKeys\" type=\"button\">Refresh<\/button>\n      <\/div>\n      <div class=\"tbl\"><table><thead><tr><th>Key<\/th><th>Status<\/th><th>Note<\/th><th>PC<\/th><th>Made<\/th><th style=\"text-align:right\">Actions<\/th><\/tr><\/thead><tbody id=\"keyRows\"><\/tbody><\/table><\/div>\n    <\/section>\n\n    <section id=\"tab-sales\" class=\"hide\">\n      <div class=\"stats\" id=\"saleStats\"><\/div>\n      <div class=\"tbl\"><table><thead><tr><th>Date<\/th><th>Email<\/th><th>Paid<\/th><th>Key<\/th><\/tr><\/thead><tbody id=\"saleRows\"><\/tbody><\/table><\/div>\n      <div class=\"bar\" style=\"margin-top:14px\"><button class=\"g\" id=\"syncSales\" type=\"button\">Find missing orders<\/button><span class=\"muted small\" id=\"syncStatus\">Checks your last 100 Stripe payments and makes a key for any that don't have one (e.g. the buyer closed the tab straight after paying).<\/span><\/div>\n      <p class=\"muted small\" style=\"margin-top:6px\">Every Stripe payment makes a key automatically. If a buyer lost theirs, find their email here and send them the key. Refunds and chargebacks disable the key automatically when the Stripe webhook is set up.<\/p>\n    <\/section>\n\n    <section id=\"tab-release\" class=\"hide\">\n      <div class=\"grid2\">\n        <div class=\"card\">\n          <h3 style=\"font-size:15px;margin-bottom:12px\">Live version<\/h3>\n          <div id=\"relCur\" class=\"muted\">Nothing published yet. People download the old file until you publish one here.<\/div>\n        <\/div>\n        <div class=\"card\">\n          <h3 style=\"font-size:15px;margin-bottom:4px\">Publish a new version<\/h3>\n          <p class=\"muted small\">Everyone with the app gets \"Update now\" next time they open it, and the website download switches to this file.<\/p>\n          <label class=\"f\" for=\"relFile\">1. Pick the new AstroSpeed Tweaks.exe<\/label>\n          <input id=\"relFile\" type=\"file\" accept=\".exe\">\n          <label class=\"f\" for=\"relVer\">2. Version number (fills in by itself)<\/label>\n          <input id=\"relVer\" placeholder=\"e.g. 1.9.0\">\n          <label class=\"f\" for=\"relNotes\">3. What's new (one change per line)<\/label>\n          <textarea id=\"relNotes\" placeholder=\"- Faster boosting&#10;- New game added\"><\/textarea>\n          <button id=\"publish\" type=\"button\" style=\"width:100%;margin-top:14px\">Upload and publish<\/button>\n          <div class=\"prog hide\" id=\"relProg\"><i id=\"relBar\"><\/i><\/div>\n          <div class=\"status info\" id=\"relStatus\"><\/div>\n        <\/div>\n      <\/div>\n    <\/section>\n\n    <section id=\"tab-pack\" class=\"hide\">\n      <div class=\"grid2\">\n        <div class=\"card\">\n          <h3 style=\"font-size:15px;margin-bottom:4px\">Your .bat tweaks<\/h3>\n          <p class=\"muted small\">Each .bat becomes a switch in the app's Live section for people with a key. No app update needed.<\/p>\n          <div id=\"batList\" style=\"margin-top:10px\"><\/div>\n          <label class=\"f\" for=\"batFiles\">Add .bat files<\/label>\n          <input id=\"batFiles\" type=\"file\" accept=\".bat,.cmd\" multiple>\n          <details style=\"margin-top:12px\"><summary class=\"small\" style=\"cursor:pointer;color:var(--ink2)\">Or paste one<\/summary>\n            <label class=\"f\" for=\"batName\">Name<\/label><input id=\"batName\" placeholder=\"my_fps_tweaks.bat\">\n            <label class=\"f\" for=\"batText\">Contents<\/label><textarea id=\"batText\" placeholder=\"reg add ...\"><\/textarea>\n            <button class=\"g\" id=\"batAdd\" type=\"button\" style=\"margin-top:10px\">Add it<\/button>\n          <\/details>\n        <\/div>\n        <div class=\"card\">\n          <h3 style=\"font-size:15px;margin-bottom:4px\">Message to everyone<\/h3>\n          <p class=\"muted small\">Shown at the top of the app until each person dismisses it. Leave empty for none.<\/p>\n          <input id=\"packMsg\" maxlength=\"400\" placeholder=\"e.g. New Fortnite season tweaks added!\">\n          <details style=\"margin-top:14px\"><summary class=\"small\" style=\"cursor:pointer;color:var(--ink2)\">Advanced: astrospeed.json<\/summary>\n            <p class=\"muted small\" style=\"margin:8px 0\">Extra sections, tweaks, games and pictures in the online pack format. Leave empty if you only use .bat files.<\/p>\n            <textarea id=\"packJson\" style=\"min-height:180px\"><\/textarea>\n          <\/details>\n          <button id=\"packSave\" type=\"button\" style=\"width:100%;margin-top:14px\">Save live tweaks<\/button>\n          <div class=\"status info\" id=\"packStatus\"><\/div>\n        <\/div>\n      <\/div>\n    <\/section>\n\n    <section id=\"tab-settings\" class=\"hide\">\n      <div class=\"grid2\">\n        <div class=\"card\">\n          <h3 style=\"font-size:15px;margin-bottom:4px\">Shop<\/h3>\n          <label class=\"f\" for=\"sName\">Product name<\/label><input id=\"sName\">\n          <label class=\"f\" for=\"sPrice\">Price of a key<\/label><input id=\"sPrice\" type=\"number\" step=\"0.01\" min=\"0\">\n          <label class=\"f\" for=\"sCur\">Currency<\/label>\n          <select id=\"sCur\"><option value=\"aud\">AUD (Australian dollar)<\/option><option value=\"usd\">USD (US dollar)<\/option><option value=\"nzd\">NZD<\/option><option value=\"gbp\">GBP<\/option><option value=\"eur\">EUR<\/option><option value=\"cad\">CAD<\/option><\/select>\n          <label class=\"f\" for=\"sMail\">Support email shown on the website (optional)<\/label><input id=\"sMail\" type=\"email\" placeholder=\"you@example.com\">\n          <button id=\"saveSettings\" type=\"button\" style=\"width:100%;margin-top:14px\">Save<\/button>\n          <div class=\"status info\" id=\"setStatus\"><\/div>\n        <\/div>\n        <div class=\"card\">\n          <h3 style=\"font-size:15px;margin-bottom:10px\">Your links<\/h3>\n          <div class=\"links\" id=\"links\"><\/div>\n          <h3 style=\"font-size:15px;margin:18px 0 6px\">Payments<\/h3>\n          <div id=\"stripeInfo\" class=\"small muted\"><\/div>\n        <\/div>\n      <\/div>\n    <\/section>\n  <\/div>\n<\/div>\n\n<script>\n(function () {\n  \"use strict\";\n  var PASS = \"\";\n  var KEYS = [], SALES = [], PACK = { message: \"\", json: \"\", bats: {} }, STATE = {};\n  function $(id) { return document.getElementById(id); }\n  function esc(s) { return String(s == null ? \"\" : s).replace(/[&<>\"']/g, function (c) { return { \"&\": \"&amp;\", \"<\": \"&lt;\", \">\": \"&gt;\", '\"': \"&quot;\", \"'\": \"&#39;\" }[c]; }); }\n  function show(id, on) { $(id).classList.toggle(\"hide\", !on); }\n  function setStatus(id, text, kind) { var e = $(id); e.textContent = text || \"\"; e.className = \"status \" + (kind || \"info\"); }\n  function fmtDate(t) { if (!t) return \"\"; var d = new Date(t); return d.toLocaleDateString() + \" \" + d.toLocaleTimeString([], { hour: \"2-digit\", minute: \"2-digit\" }); }\n  function money(c, cur) { return ((Number(c) || 0) / 100).toFixed(2) + \" \" + String(cur || \"\").toUpperCase(); }\n  function toast(t) { var d = document.createElement(\"div\"); d.className = \"toast\"; d.textContent = t; document.body.appendChild(d); setTimeout(function () { d.remove(); }, 2600); }\n  function copy(t) { try { navigator.clipboard.writeText(t); toast(\"Copied\"); } catch (e) { window.prompt(\"Copy this:\", t); } }\n  try { PASS = sessionStorage.getItem(\"asp2\") || \"\"; } catch (e) {}\n\n  function api(action, extra) {\n    var p = new Promise(function (res, rej) {\n      var ctl = window.AbortController ? new AbortController() : null;\n      var timer = setTimeout(function () { if (ctl) ctl.abort(); }, 20000);\n      var body = { pass: PASS, action: action };\n      if (extra) for (var k in extra) body[k] = extra[k];\n      fetch(\"/admin\", { method: \"POST\", headers: { \"Content-Type\": \"application/json\" }, body: JSON.stringify(body), signal: ctl ? ctl.signal : undefined })\n        .then(function (r) {\n          return r.text().then(function (t) {\n            clearTimeout(timer);\n            var d;\n            try { d = JSON.parse(t); } catch (e) { return rej(new Error(\"The server sent something unexpected (HTTP \" + r.status + \"). Is the newest code deployed?\")); }\n            if (!d.ok) { var err = new Error(d.reason || (\"HTTP \" + r.status)); err.code = d.code; return rej(err); }\n            res(d);\n          });\n        }, function (e) {\n          clearTimeout(timer);\n          rej(new Error(e && e.name === \"AbortError\" ? \"The server took too long to answer.\" : \"Couldn't reach the server. Check your internet.\"));\n        });\n    });\n    return p;\n  }\n\n  // ── setup checklist (works before signing in) ──\n  function renderChecks(h) {\n    if (!h) return;\n    var items = [\n      [h.storage ? \"ok\" : \"bad\", h.storage ? \"Storage connected\" : \"Storage NOT connected (add the LICENSES binding)\"],\n      [h.passwordStrong ? \"ok\" : (h.password ? \"bad\" : \"bad\"), h.passwordStrong ? \"Password set\" : (h.password ? \"Password too short (needs \" + (h.minPassword || 12) + \"+ characters)\" : \"No ADMIN_PASS set\")],\n      [h.stripe ? (h.stripeTest ? \"warn\" : \"ok\") : \"warn\", h.stripe ? (h.stripeTest ? \"Stripe in TEST mode\" : \"Stripe connected\") : \"Payments off (no STRIPE_SECRET_KEY)\"],\n      [h.webhook ? \"ok\" : (h.stripe ? \"warn\" : \"\"), h.webhook ? \"Stripe webhook on\" : \"Stripe webhook not set (recommended)\"]\n    ];\n    $(\"checks\").innerHTML = items.map(function (i) { return '<div class=\"chk ' + i[0] + '\"><i><\/i>' + esc(i[1]) + \"<\/div>\"; }).join(\"\");\n  }\n  fetch(\"/health\").then(function (r) { return r.json(); }).then(function (h) { STATE.health = h; renderChecks(h); })\n    .catch(function () { $(\"checks\").innerHTML = '<div class=\"chk bad\"><i><\/i>Couldn’t reach the server<\/div>'; });\n\n  // ── sign in ──\n  function signedIn(d) {\n    STATE.health = d.health; STATE.settings = d.settings; STATE.release = d.release;\n    renderChecks(d.health);\n    show(\"loginForm\", false); show(\"app\", true); show(\"signOut\", true);\n    try { sessionStorage.setItem(\"asp2\", PASS); } catch (e) {}\n    fillSettings(); renderRelease(); renderLinks();\n    loadKeys(); loadSales(); loadPack();\n  }\n  $(\"loginForm\").addEventListener(\"submit\", function (e) {\n    e.preventDefault();\n    PASS = $(\"pw\").value;\n    if (!PASS) { setStatus(\"loginStatus\", \"Type your password first.\", \"err\"); return; }\n    $(\"go\").disabled = true;\n    setStatus(\"loginStatus\", \"Checking…\", \"info\");\n    api(\"login\").then(function (d) { setStatus(\"loginStatus\", \"\", \"info\"); signedIn(d); })\n      .catch(function (err) { setStatus(\"loginStatus\", err.message, \"err\"); })\n      .then(function () { $(\"go\").disabled = false; });\n  });\n  $(\"signOut\").addEventListener(\"click\", function () { try { sessionStorage.removeItem(\"asp2\"); } catch (e) {} location.reload(); });\n  if (PASS) {\n    setStatus(\"loginStatus\", \"Signing you back in…\", \"info\");\n    api(\"login\").then(signedIn).catch(function (err) { PASS = \"\"; setStatus(\"loginStatus\", err.message, \"err\"); });\n  }\n\n  // ── tabs ──\n  [].forEach.call(document.querySelectorAll(\".tab\"), function (b) {\n    b.addEventListener(\"click\", function () {\n      [].forEach.call(document.querySelectorAll(\".tab\"), function (x) { x.classList.toggle(\"on\", x === b); });\n      [\"keys\", \"sales\", \"release\", \"pack\", \"settings\"].forEach(function (t) { show(\"tab-\" + t, t === b.getAttribute(\"data-tab\")); });\n    });\n  });\n\n  // ── keys ──\n  function statusOf(r) { return r.revoked ? \"revoked\" : (r.expiresAt && Date.now() >= r.expiresAt) ? \"expired\" : (r.hwid ? \"active\" : \"unused\"); }\n  function left(ms) { var d = ms - Date.now(); if (d <= 0) return \"ended\"; var h = Math.floor(d / 36e5); return h >= 24 ? Math.floor(h / 24) + \"d \" + (h % 24) + \"h left\" : h >= 1 ? h + \"h left\" : Math.ceil(d / 6e4) + \" min left\"; }\n  function trialTag(k) {\n    if (k.expiresAt) return ' <span class=\"tag t-trial\">trial · ' + esc(left(k.expiresAt)) + \"<\/span>\";\n    if (k.trialMs) return ' <span class=\"tag t-trial\">' + Math.round(k.trialMs / 864e5) + \"-day trial<\/span>\";\n    return \"\";\n  }\n  var FRESH = {}; // keys made in this session, shown even before Cloudflare's list catches up\n  function loadKeys() { return api(\"keys\").then(function (d) {\n    KEYS = d.rows || [];\n    var have = {}; KEYS.forEach(function (k) { have[k.key] = 1; });\n    Object.keys(FRESH).forEach(function (k) { if (!have[k] && !FRESH[k].deleted) KEYS.unshift(FRESH[k]); });\n    drawKeys(); }).catch(function (e) { $(\"keyRows\").innerHTML = '<tr><td colspan=\"6\" class=\"empty\">' + esc(e.message) + \"<\/td><\/tr>\"; }); }\n  function drawKeys() {\n    var a = 0, u = 0, r = 0;\n    KEYS.forEach(function (k) { var s = statusOf(k); if (s === \"active\") a++; else if (s === \"unused\") u++; else r++; });\n    $(\"keyStats\").innerHTML = '<div class=\"stat\"><b>' + KEYS.length + '<\/b><span>Total keys<\/span><\/div><div class=\"stat a\"><b>' + a + '<\/b><span>Activated<\/span><\/div><div class=\"stat u\"><b>' + u + '<\/b><span>Not used yet<\/span><\/div><div class=\"stat r\"><b>' + r + '<\/b><span>Disabled<\/span><\/div>';\n    var q = ($(\"search\").value || \"\").toLowerCase();\n    var rows = KEYS.filter(function (k) { return !q || (k.key + \" \" + (k.hwid || \"\") + \" \" + (k.note || \"\")).toLowerCase().indexOf(q) >= 0; });\n    if (!rows.length) { $(\"keyRows\").innerHTML = '<tr><td colspan=\"6\" class=\"empty\">' + (KEYS.length ? \"Nothing matches that search.\" : \"No keys yet. Click “+ Make keys”.\") + \"<\/td><\/tr>\"; return; }\n    $(\"keyRows\").innerHTML = rows.map(function (k) {\n      var s = statusOf(k), key = esc(k.key);\n      var acts = '<div class=\"acts\">' +\n        (k.revoked ? '<button class=\"sm g\" data-a=\"enable\" data-k=\"' + key + '\" type=\"button\">Enable<\/button>' : '<button class=\"sm g\" data-a=\"revoke\" data-k=\"' + key + '\" type=\"button\">Disable<\/button>') +\n        (k.hwid ? '<button class=\"sm g\" data-a=\"unbind\" data-k=\"' + key + '\" type=\"button\" title=\"Lets the key be activated on a different PC\">Reset PC<\/button>' : \"\") +\n        ((k.trialMs || k.expiresAt) ? '<button class=\"sm g\" data-a=\"extend\" data-k=\"' + key + '\" type=\"button\" title=\"Add 7 more days\">+7 days<\/button><button class=\"sm g\" data-a=\"makeFull\" data-k=\"' + key + '\" type=\"button\" title=\"Turn this trial into a full key\">Make full<\/button>' : \"\") +\n        '<button class=\"sm dz\" data-a=\"delete\" data-k=\"' + key + '\" type=\"button\">Delete<\/button><\/div>';\n      return \"<tr><td><span class=\\\"k\\\">\" + key + '<\/span><button class=\"cp\" data-cp=\"' + key + '\" type=\"button\" title=\"Copy\">copy<\/button><\/td>' +\n        '<td><span class=\"tag t-' + (s === \"expired\" ? \"revoked\" : s) + '\">' + (s === \"unused\" ? \"not used\" : s === \"revoked\" ? \"disabled\" : s === \"expired\" ? \"trial ended\" : \"activated\") + \"<\/span>\" + trialTag(k) + \"<\/td>\" +\n        \"<td>\" + (k.note ? esc(k.note) : '<span class=\"muted\">—<\/span>') + (k.source === \"stripe\" ? ' <span class=\"tag t-active\">sold<\/span>' : \"\") + \"<\/td>\" +\n        '<td class=\"hw\" title=\"' + esc(k.hwid || \"\") + '\">' + (k.hwid ? esc(k.hwid.slice(0, 10)) + \"…\" : '<span class=\"muted\">—<\/span>') + \"<\/td>\" +\n        '<td class=\"muted small\">' + fmtDate(k.createdAt) + \"<\/td><td>\" + acts + \"<\/td><\/tr>\";\n    }).join(\"\");\n  }\n  $(\"keyRows\").addEventListener(\"click\", function (e) {\n    var b = e.target.closest(\"button\"); if (!b) return;\n    if (b.hasAttribute(\"data-cp\")) { copy(b.getAttribute(\"data-cp\")); return; }\n    var key = b.getAttribute(\"data-k\"), a = b.getAttribute(\"data-a\");\n    if (!a) return;\n    if (a === \"delete\" && !confirm(\"Delete \" + key + \" for good? The app stops working on that PC.\")) return;\n    if (a === \"revoke\" && !confirm(\"Disable \" + key + \"? The app locks on that PC next time it opens.\")) return;\n    if (a === \"unbind\" && !confirm(\"Reset \" + key + \" so it can be activated on another PC?\")) return;\n    if (a === \"makeFull\" && !confirm(\"Turn \" + key + \" into a full key that never runs out?\")) return;\n    b.disabled = true;\n    api(a, { key: key, days: 7 }).then(function (res) {\n      toast(\"Done\");\n      KEYS.forEach(function (k) { if (k.key !== key) return; if (a === \"revoke\") k.revoked = true; if (a === \"enable\") k.revoked = false; if (a === \"unbind\") k.hwid = null; if (res && res.rec) { k.expiresAt = res.rec.expiresAt; k.trialMs = res.rec.trialMs; } });\n      if (a === \"delete\") { KEYS = KEYS.filter(function (k) { return k.key !== key; }); if (FRESH[key]) FRESH[key].deleted = true; }\n      drawKeys();\n    }).catch(function (err) { alert(err.message); b.disabled = false; });\n  });\n  $(\"search\").addEventListener(\"input\", drawKeys);\n  $(\"refreshKeys\").addEventListener(\"click\", loadKeys);\n  $(\"gen\").addEventListener(\"click\", function () {\n    var b = $(\"gen\"); b.disabled = true;\n    var trialDays = Number($(\"kind\").value) || 0;\n    api(\"generate\", { count: $(\"count\").value, note: $(\"note\").value, trialDays: trialDays }).then(function (d) {\n      var list = d.made.join(\"\\n\");\n      d.made.forEach(function (k) { FRESH[k] = { key: k, createdAt: Date.now(), hwid: null, revoked: false, note: $(\"note\").value, source: \"admin\", trialMs: trialDays * 86400000 }; });\n      $(\"made\").innerHTML = '<div class=\"made\"><b>New ' + (trialDays ? trialDays + \"-day trial key\" : \"key\") + (d.made.length > 1 ? \"s\" : \"\") + '<\/b>' + (trialDays ? \" (the clock starts when it's activated)\" : \"\") + ' — copy and send to the person:<div class=\"keys\">' + esc(list) + '<\/div><button class=\"sm\" id=\"cpAll\" type=\"button\">Copy<\/button><\/div>';\n      $(\"cpAll\").addEventListener(\"click\", function () { copy(list); });\n      $(\"note\").value = \"\";\n      return loadKeys();\n    }).catch(function (err) { alert(err.message); }).then(function () { b.disabled = false; });\n  });\n\n  // ── sales ──\n  function loadSales() {\n    return api(\"sales\").then(function (d) {\n      SALES = d.rows || [];\n      var total = {}, n = SALES.length;\n      SALES.forEach(function (s) { var c = (s.currency || \"\").toUpperCase(); total[c] = (total[c] || 0) + (s.amount || 0); });\n      var rev = Object.keys(total).map(function (c) { return ((total[c] || 0) / 100).toFixed(2) + \" \" + c; }).join(\" + \") || \"0.00\";\n      var last = SALES[0] ? fmtDate(SALES[0].createdAt) : \"—\";\n      $(\"saleStats\").innerHTML = '<div class=\"stat\"><b>' + n + '<\/b><span>Keys sold<\/span><\/div><div class=\"stat a\" style=\"grid-column:span 2\"><b>' + esc(rev) + '<\/b><span>Total taken (before Stripe fees)<\/span><\/div><div class=\"stat\"><b style=\"font-size:15px;padding-top:8px\">' + esc(last) + \"<\/b><span>Last sale<\/span><\/div>\";\n      $(\"saleRows\").innerHTML = SALES.length ? SALES.map(function (s) {\n        return '<tr><td class=\"small\">' + fmtDate(s.createdAt) + \"<\/td><td>\" + esc(s.email || \"—\") + \"<\/td><td>\" + money(s.amount, s.currency) + '<\/td><td><span class=\"k\">' + esc(s.key) + '<\/span><button class=\"cp\" data-cp=\"' + esc(s.key) + '\" type=\"button\">copy<\/button><\/td><\/tr>';\n      }).join(\"\") : '<tr><td colspan=\"4\" class=\"empty\">No sales yet.<\/td><\/tr>';\n    }).catch(function (e) { $(\"saleRows\").innerHTML = '<tr><td colspan=\"4\" class=\"empty\">' + esc(e.message) + \"<\/td><\/tr>\"; });\n  }\n  $(\"syncSales\").addEventListener(\"click\", function () {\n    var b = $(\"syncSales\"); b.disabled = true; $(\"syncStatus\").textContent = \"Checking Stripe…\";\n    api(\"syncSales\").then(function (d) {\n      $(\"syncStatus\").textContent = d.made ? (\"Found \" + d.made + \" order(s) without a key and made them. They're in the list now.\") : (\"All \" + d.checked + \" recent paid orders already have keys.\");\n      return Promise.all([loadSales(), loadKeys()]);\n    }).catch(function (err) { $(\"syncStatus\").textContent = err.message; }).then(function () { b.disabled = false; });\n  });\n  $(\"saleRows\").addEventListener(\"click\", function (e) { var b = e.target.closest(\"[data-cp]\"); if (b) copy(b.getAttribute(\"data-cp\")); });\n\n  // ── release ──\n  function renderRelease() {\n    var r = STATE.release;\n    if (!r) { $(\"relCur\").className = \"muted\"; $(\"relCur\").innerHTML = 'Nothing published yet. People download the old file until you publish one here.'; return; }\n    $(\"relCur\").className = \"\";\n    $(\"relCur\").innerHTML = '<div class=\"cur\"><b>' + esc(r.version) + '<\/b><span class=\"muted\">' + (r.size ? (r.size / 1048576).toFixed(1) + \" MB · \" : \"\") + \"published \" + esc(fmtDate(r.publishedAt)) + \"<\/span><\/div>\" +\n      (r.notes ? '<pre style=\"white-space:pre-wrap;margin-top:12px;color:var(--ink2);font-family:inherit\">' + esc(r.notes) + \"<\/pre>\" : \"\") +\n      '<p style=\"margin-top:12px\"><a href=\"/download\" target=\"_blank\" rel=\"noopener\">Test the download<\/a><\/p>';\n    var parts = String(r.version).split(\".\").map(Number);\n    if (parts.length >= 3 && !$(\"relVer\").value) { parts[2] = (parts[2] || 0) + 1; $(\"relVer\").placeholder = \"e.g. \" + parts.join(\".\"); }\n  }\n  function newer(a, b) { var x = String(a).split(\".\").map(Number), y = String(b).split(\".\").map(Number); for (var i = 0; i < Math.max(x.length, y.length); i++) { var p = x[i] || 0, q = y[i] || 0; if (p !== q) return p > q; } return false; }\n  $(\"relFile\").addEventListener(\"change\", function () {\n    var f = $(\"relFile\").files[0]; if (!f) return;\n    setStatus(\"relStatus\", \"Reading the file…\", \"info\");\n    var rd = new FileReader();\n    rd.onload = function () {\n      var bytes = new Uint8Array(rd.result), mark = \"ASTROSPEED_VERSION=\", txt = \"\";\n      // scan as plain bytes (fast, no decoding of 8 MB)\n      var m0 = mark.charCodeAt(0);\n      for (var i = 0; i < bytes.length - 40; i++) {\n        if (bytes[i] !== m0) continue;\n        var ok = true; for (var j = 1; j < mark.length; j++) if (bytes[i + j] !== mark.charCodeAt(j)) { ok = false; break; }\n        if (!ok) continue;\n        for (var e = i + mark.length; e < i + mark.length + 20 && bytes[e] !== 59; e++) txt += String.fromCharCode(bytes[e]);\n        break;\n      }\n      if (bytes[0] !== 0x4d || bytes[1] !== 0x5a) { setStatus(\"relStatus\", \"That isn't a Windows .exe file.\", \"err\"); return; }\n      if (/^\\d+(\\.\\d+){1,3}$/.test(txt)) {\n        $(\"relVer\").value = txt;\n        if (STATE.release && !newer(txt, STATE.release.version)) setStatus(\"relStatus\", \"This file is version \" + txt + \", which isn't newer than the live \" + STATE.release.version + \". Pick the new build.\", \"err\");\n        else setStatus(\"relStatus\", \"Found version \" + txt + \" in the file. Add what's new, then click Upload and publish.\", \"ok\");\n      } else setStatus(\"relStatus\", \"Couldn't read a version from this file (it may be older than 1.9.0). Type the version yourself.\", \"info\");\n    };\n    rd.readAsArrayBuffer(f);\n  });\n\n  $(\"publish\").addEventListener(\"click\", function () {\n    var ver = $(\"relVer\").value.trim().replace(/^v/i, \"\"), f = $(\"relFile\").files[0], notes = $(\"relNotes\").value;\n    if (!/^\\d+(\\.\\d+){1,3}$/.test(ver)) { setStatus(\"relStatus\", \"Type a version number like 1.9.0\", \"err\"); return; }\n    if (STATE.release && !newer(ver, STATE.release.version)) { setStatus(\"relStatus\", \"The version has to be higher than \" + STATE.release.version + \" or nobody gets the update.\", \"err\"); return; }\n    if (!f) { setStatus(\"relStatus\", \"Choose the .exe file first.\", \"err\"); return; }\n    if (!/\\.exe$/i.test(f.name)) { setStatus(\"relStatus\", \"That isn't an .exe file.\", \"err\"); return; }\n    if (!confirm(\"Publish version \" + ver + \"? Everyone with the app will be offered this update.\")) return;\n    var b = $(\"publish\"); b.disabled = true; show(\"relProg\", true); $(\"relBar\").style.width = \"0%\";\n    setStatus(\"relStatus\", \"Uploading…\", \"info\");\n    var xhr = new XMLHttpRequest();\n    xhr.open(\"POST\", \"/admin/upload?version=\" + encodeURIComponent(ver));\n    xhr.setRequestHeader(\"X-Admin-Pass\", PASS);\n    xhr.upload.onprogress = function (e) { if (e.lengthComputable) { var p = Math.round(e.loaded / e.total * 100); $(\"relBar\").style.width = p + \"%\"; setStatus(\"relStatus\", \"Uploading… \" + p + \"%\", \"info\"); } };\n    xhr.onerror = function () { b.disabled = false; setStatus(\"relStatus\", \"Upload failed. Check your internet and try again.\", \"err\"); };\n    xhr.onload = function () {\n      var d = {}; try { d = JSON.parse(xhr.responseText); } catch (e) {}\n      if (!d.ok) { b.disabled = false; setStatus(\"relStatus\", d.reason || (\"Upload failed (HTTP \" + xhr.status + \")\"), \"err\"); return; }\n      setStatus(\"relStatus\", \"Publishing…\", \"info\");\n      api(\"publish\", { version: ver, notes: notes }).then(function (r) {\n        $(\"relVer\").value = \"\"; $(\"relNotes\").value = \"\"; $(\"relFile\").value = \"\";\n        STATE.release = r.release; renderRelease(); renderLinks();\n        setStatus(\"relStatus\", \"Version \" + ver + \" is live. The app will offer it to everyone.\", \"ok\");\n      }).catch(function (err) { setStatus(\"relStatus\", err.message, \"err\"); }).then(function () { b.disabled = false; });\n    };\n    xhr.send(f);\n  });\n\n  // ── live tweaks ──\n  function loadPack() { return api(\"getPack\").then(function (d) { PACK = d.pack || PACK; PACK.bats = PACK.bats || {}; $(\"packMsg\").value = PACK.message || \"\"; $(\"packJson\").value = PACK.json || \"\"; drawBats(); }).catch(function () {}); }\n  function drawBats() {\n    var names = Object.keys(PACK.bats).sort();\n    $(\"batList\").innerHTML = names.length ? names.map(function (n) { return '<div class=\"batrow\"><span class=\"n\">' + esc(n) + '<\/span><span class=\"muted small\">' + Math.ceil((PACK.bats[n] || \"\").length / 1024) + ' KB<\/span><button class=\"sm dz\" data-del=\"' + esc(n) + '\" type=\"button\">Remove<\/button><\/div>'; }).join(\"\") : '<p class=\"muted small\">No .bat tweaks yet.<\/p>';\n  }\n  $(\"batList\").addEventListener(\"click\", function (e) { var b = e.target.closest(\"[data-del]\"); if (!b) return; delete PACK.bats[b.getAttribute(\"data-del\")]; drawBats(); setStatus(\"packStatus\", \"Removed. Click Save to make it live.\", \"info\"); });\n  $(\"batFiles\").addEventListener(\"change\", function () {\n    var files = [].slice.call($(\"batFiles\").files), left = files.length;\n    files.forEach(function (f) { var rd = new FileReader(); rd.onload = function () { PACK.bats[f.name] = String(rd.result || \"\"); if (--left === 0) { drawBats(); setStatus(\"packStatus\", files.length + \" file(s) added. Click Save to make them live.\", \"info\"); } }; rd.readAsText(f); });\n    $(\"batFiles\").value = \"\";\n  });\n  $(\"batAdd\").addEventListener(\"click\", function () {\n    var n = $(\"batName\").value.trim(), t = $(\"batText\").value;\n    if (!n || !t.trim()) { setStatus(\"packStatus\", \"Give it a name and paste the contents.\", \"err\"); return; }\n    if (!/\\.(bat|cmd)$/i.test(n)) n += \".bat\";\n    PACK.bats[n] = t; $(\"batName\").value = \"\"; $(\"batText\").value = \"\"; drawBats();\n    setStatus(\"packStatus\", n + \" added. Click Save to make it live.\", \"info\");\n  });\n  $(\"packSave\").addEventListener(\"click\", function () {\n    var b = $(\"packSave\"); b.disabled = true; setStatus(\"packStatus\", \"Saving…\", \"info\");\n    api(\"savePack\", { pack: { message: $(\"packMsg\").value, json: $(\"packJson\").value, bats: PACK.bats } }).then(function (d) {\n      PACK = d.pack; drawBats(); setStatus(\"packStatus\", \"Saved. The app picks this up within 30 minutes (or right away with Check now).\", \"ok\");\n    }).catch(function (err) { setStatus(\"packStatus\", err.message, \"err\"); }).then(function () { b.disabled = false; });\n  });\n\n  // ── settings ──\n  function fillSettings() {\n    var s = STATE.settings || {};\n    $(\"sName\").value = s.productName || \"\"; $(\"sPrice\").value = ((Number(s.price) || 0) / 100).toFixed(2);\n    $(\"sCur\").value = s.currency || \"aud\"; $(\"sMail\").value = s.supportEmail || \"\";\n    var h = STATE.health || {};\n    $(\"stripeInfo\").innerHTML = h.stripe\n      ? (h.stripeTest ? \"Stripe is connected in <b>test mode<\/b>: no real money moves. Pay with card 4242 4242 4242 4242, any future date and any CVC to try it. Swap to your live key (sk_live_…) when you're ready.\" : \"Stripe is connected. Buyers pay on Stripe and get their key on screen straight away. Money goes to your Stripe balance.\")\n      : \"Payments are off. To switch them on, add your Stripe secret key to the worker as a Secret named <b>STRIPE_SECRET_KEY<\/b> (the START HERE guide walks you through it).\";\n  }\n  function renderLinks() {\n    var o = location.origin;\n    var rows = [[\"Website\", o + \"/\"], [\"Buy a key\", o + \"/buy\"], [\"Download\", o + \"/download\"], [\"Admin\", o + \"/admin\"]];\n    $(\"links\").innerHTML = rows.map(function (r) { return \"<div><span>\" + r[0] + \"<\/span><code>\" + esc(r[1]) + '<\/code><button class=\"cp\" data-cp=\"' + esc(r[1]) + '\" type=\"button\">copy<\/button><\/div>'; }).join(\"\");\n  }\n  $(\"links\").addEventListener(\"click\", function (e) { var b = e.target.closest(\"[data-cp]\"); if (b) copy(b.getAttribute(\"data-cp\")); });\n  $(\"saveSettings\").addEventListener(\"click\", function () {\n    var price = Math.round(parseFloat($(\"sPrice\").value || \"0\") * 100);\n    var b = $(\"saveSettings\"); b.disabled = true; setStatus(\"setStatus\", \"Saving…\", \"info\");\n    api(\"saveSettings\", { settings: { productName: $(\"sName\").value, price: price, currency: $(\"sCur\").value, supportEmail: $(\"sMail\").value } })\n      .then(function (d) { STATE.settings = d.settings; fillSettings(); setStatus(\"setStatus\", \"Saved. The website shows the new price straight away.\", \"ok\"); })\n      .catch(function (err) { setStatus(\"setStatus\", err.message, \"err\"); }).then(function () { b.disabled = false; });\n  });\n})();\n<\/script>\n<\/body>\n<\/html>\n";
const THANKS_HTML = "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<meta name=\"robots\" content=\"noindex\">\n<title>Your AstroSpeed key<\/title>\n<style>\n*{box-sizing:border-box;margin:0;padding:0}\n:root{--bg:#0B0B0C;--card:#17171A;--card2:#1E1E22;--ink:#F3F1EC;--ink2:#A6A8AE;--ink3:#6B6E74;--line:rgba(255,255,255,.09);--accent:#FF6B3F;--accent2:#FFB27A;--accent3:#FF3D6E;--ok:#4FC27E}\nhtml,body{background:var(--bg);color:var(--ink);font-family:\"Segoe UI\",system-ui,sans-serif}\nbody{min-height:100vh;display:flex;align-items:center;justify-content:center;padding:28px 16px;position:relative;overflow-x:hidden}\n.glow{position:fixed;top:-180px;left:50%;transform:translateX(-50%);width:760px;height:520px;background:radial-gradient(ellipse at center,rgba(255,107,63,.22),transparent 65%);filter:blur(20px);pointer-events:none}\n.card{position:relative;max-width:560px;width:100%;background:var(--card);border:1px solid var(--line);border-radius:24px;padding:34px 30px;text-align:center;box-shadow:0 50px 120px -50px rgba(0,0,0,.9);animation:in .8s cubic-bezier(.16,1,.3,1)}\n@keyframes in{from{opacity:0;transform:translateY(24px) scale(.98)}}\n.tick{width:64px;height:64px;margin:0 auto 18px;border-radius:50%;background:rgba(79,194,126,.14);display:grid;place-items:center}\n.tick svg{width:34px;height:34px}\n.tick path{stroke:var(--ok);stroke-width:3;fill:none;stroke-linecap:round;stroke-linejoin:round;stroke-dasharray:40;stroke-dashoffset:40;animation:draw .7s .3s cubic-bezier(.16,1,.3,1) forwards}\n@keyframes draw{to{stroke-dashoffset:0}}\nh1{font-size:28px;letter-spacing:-.03em;font-weight:800;margin-bottom:8px}\np{color:var(--ink2);line-height:1.55}\n.key{margin:22px 0 10px;padding:18px 14px;border-radius:16px;border:1.5px dashed rgba(255,107,63,.55);background:rgba(255,107,63,.07);font-family:ui-monospace,Consolas,monospace;font-size:clamp(17px,4.6vw,24px);font-weight:800;letter-spacing:.04em;word-break:break-all;user-select:all}\n.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;height:48px;padding:0 24px;border-radius:999px;font-weight:800;font-size:15px;border:0;cursor:pointer;text-decoration:none}\n.btn.primary{background:linear-gradient(120deg,var(--accent3),var(--accent) 55%,var(--accent2));color:#1b0a04}\n.btn.ghost{background:transparent;color:var(--ink);border:1.5px solid var(--line)}\n.row{display:flex;gap:10px;justify-content:center;flex-wrap:wrap;margin-top:6px}\n.warn{font-size:13px;color:#F2B347;margin-top:12px;font-weight:600}\nol{text-align:left;margin:26px auto 0;max-width:420px;color:var(--ink2);line-height:1.7;padding-left:20px}\nol b{color:var(--ink)}\n<\/style>\n<\/head>\n<body>\n<div class=\"glow\"><\/div>\n<div class=\"card\">\n  <div class=\"tick\"><svg viewBox=\"0 0 24 24\"><path d=\"m5 12.5 4.5 4.5L19 7.5\"/><\/svg><\/div>\n  <h1>Thanks! Here's your key<\/h1>\n  <p>Payment received. This key unlocks the full version on one PC.<\/p>\n  <div class=\"key\" id=\"key\">{{KEY}}<\/div>\n  <div class=\"row\">\n    <button class=\"btn primary\" id=\"copy\" type=\"button\">Copy key<\/button>\n    <a class=\"btn ghost\" href=\"{{ORIGIN}}/download\">Download AstroSpeed<\/a>\n  <\/div>\n  <p class=\"warn\">Save this key somewhere safe. You can come back to this page with the same link.<\/p>\n  <ol>\n    <li>Open <b>AstroSpeed Tweaks<\/b> (download it above if you haven't).<\/li>\n    <li>Click <b>Free version · Unlock<\/b> at the bottom-left, or go to <b>Settings › Your key<\/b>.<\/li>\n    <li>Paste the key and click <b>Activate<\/b>. Done.<\/li>\n  <\/ol>\n<\/div>\n<script>\n(function () {\n  var b = document.getElementById(\"copy\"), k = document.getElementById(\"key\").textContent.trim();\n  b.addEventListener(\"click\", function () {\n    function done() { b.textContent = \"Copied!\"; setTimeout(function () { b.textContent = \"Copy key\"; }, 1800); }\n    try { navigator.clipboard.writeText(k).then(done, function () { window.prompt(\"Copy your key:\", k); }); }\n    catch (e) { window.prompt(\"Copy your key:\", k); }\n  });\n})();\n<\/script>\n<\/body>\n<\/html>\n";
const SIMPLE_HTML = "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<title>{{TITLE}} · AstroSpeed<\/title>\n<style>\n*{box-sizing:border-box;margin:0;padding:0}\nhtml,body{background:#0B0B0C;color:#F3F1EC;font-family:\"Segoe UI\",system-ui,sans-serif}\nbody{min-height:100vh;display:flex;align-items:center;justify-content:center;padding:28px 16px}\n.card{max-width:480px;width:100%;background:#17171A;border:1px solid rgba(255,255,255,.09);border-radius:22px;padding:32px 28px;text-align:center}\nh1{font-size:24px;letter-spacing:-.02em;margin-bottom:10px}\np{color:#A6A8AE;line-height:1.55}\na.btn{display:inline-flex;margin-top:22px;height:46px;align-items:center;padding:0 22px;border-radius:999px;background:linear-gradient(120deg,#FF3D6E,#FF6B3F 55%,#FFB27A);color:#1b0a04;font-weight:800;text-decoration:none}\n<\/style>\n<\/head>\n<body>\n<div class=\"card\">\n  <h1>{{TITLE}}<\/h1>\n  {{BODY}}\n  <a class=\"btn\" href=\"{{ORIGIN}}/\">Back to the website<\/a>\n<\/div>\n<\/body>\n<\/html>\n";
