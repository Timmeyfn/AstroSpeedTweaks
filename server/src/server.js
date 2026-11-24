// ════════════════════════════════════════════════════════════════════════════
//  Astro Tweaks server — ONE file for everything. Paste this whole file into your
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
const ROLE = "all";
const KEY_ROUTES = new Set(["/activate", "/check", "/health", "/admin", "/admin/upload"]);
// Where the app was downloaded from before you upload a version in the admin page.
const LEGACY_DOWNLOAD = "https://astrospeedtweaks.dupelication.workers.dev/AstroSpeed%20Tweaks.exe";
const EXE_NAME = "Astro Tweaks.exe";
const MIN_PASS = 12;

const DEFAULT_SETTINGS = {
  productName: "Astro Tweaks",
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
      return new Response("Astro Tweaks key server is running.", { headers: { "Content-Type": "text/plain; charset=utf-8", ...CORS } });
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
          return json(await healthFull(env));
      }
      return new Response("Not found", { status: 404, headers: CORS });
    } catch (e) {
      const msg = e && e.message ? e.message : String(e);
      console.log("error on " + path + ": " + msg);
      if (request.method === "GET" && (path === "/thanks" || path === "/buy"))
        return page(simplePage("Something went wrong", "<p>We couldn't load this page just now. Your payment (if you made one) is safe. Please refresh in a minute" + (path === "/thanks" ? " — this same link will show your key." : ".") + "</p>", url.origin), 500);
      return json({ ok: false, code: "server", reason: explainError(msg) }, 500);
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

async function healthFull(env) {
  const h = health(env);
  if (env.LICENSES) {
    try { await env.LICENSES.get("cfg:settings"); h.storageRead = true; } catch (e) { h.storageRead = false; h.storageError = explainError(String(e && e.message || e)); }
  }
  return h;
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

// Turns Cloudflare errors into something the owner can act on (shown in the app and admin page).
function explainError(msg) {
  if (/LICENSES storage/.test(msg)) return "Server setup: " + msg;
  if (/limit|quota|exceeded|429/i.test(msg))
    return "The key server hit Cloudflare's free daily limit (1,000 saves a day). It resets at 8am Perth time; activations work again then.";
  if (/KV|namespace|binding/i.test(msg)) return "The key server's storage isn't working: " + msg;
  return "The key server had a problem (" + msg.slice(0, 120) + "). Try again in a minute.";
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
  if (!isKeyName(k)) return json({ ok: false, code: "invalid", reason: "That doesn't look like a full key. It looks like ASTRO-XXXXX-XXXXX-XXXXX." });
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
  if (!isKeyName(k)) return json({ ok: false, reason: "invalid" });
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
      return page(simplePage("Order not found", "<p>That link isn't an Astro Tweaks order.</p>", url.origin), 404);
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
        // made by an older server version: read it (no write here — free plan has 1,000 writes/day)
        const rec = await env.LICENSES.get(it.name, "json");
        if (rec) rows.push({ key: it.name, ...rec });
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
const SITE_HTML = __SITE_HTML__;
const ADMIN_HTML = __ADMIN_HTML__;
const THANKS_HTML = __THANKS_HTML__;
const SIMPLE_HTML = __SIMPLE_HTML__;
