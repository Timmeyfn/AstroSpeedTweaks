# Astro Tweaks — handover for a new Claude chat

Paste this whole file into a new chat with Claude (or attach it), then say what you want next.
It explains the project, how everything is set up, what's done, and the rules we agreed on.
No passwords or secret keys are in here, and none should ever be pasted into a chat.

---

## 1. What the project is

**Astro Tweaks** (formerly "AstroSpeed Tweaks") is a Windows game-optimizer app for gamers, mainly Fortnite players.

- **Owner:** Tommy (GitHub `Timmeyfn`), based in Perth, Australia (AWST, UTC+8).
- **Business model:** freemium. The free app is a teaser. A paid **key** (one-time **$20 AUD**, one key = one PC) unlocks everything.
- **Current app version:** **2.7.0**.
- **Source code:** GitHub repo **`Timmeyfn/astrospeedtweaks`**, branch `main`. The repo should be **private**. The website and the app must **never mention or link to GitHub**; the owner doesn't want customers knowing about it.
- **The owner isn't a programmer.** Keep explanations plain and give click-by-click steps. The owner deploys Cloudflare workers **manually** (copy-paste), so new server code is sent as a whole `.js` file to paste in.

## 2. Tech overview

- **App:** Go + WebView2 (`main.go` and friends). The UI is plain HTML/CSS/JS in `ui/src/` (`index.html`, `app.js`, `app.css`, `mock.js` for the browser preview). Build steps:
  - `python3 build_ui.py` writes `ui/dist/index.html`, which is embedded in the exe.
  - `python3 build_ui.py --preview` writes a browser preview using `mock.js`.
- **Windows build** (from Linux or Windows):
  ```
  go-winres make --in winres/winres.json --out rsrc   (then delete rsrc_windows_386.syso)
  GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o "Astro Tweaks.exe" .
  ```
  On Windows, `build.bat` does all of this.
- **Version number:** `const AppVersion` in `main.go`, plus `winres/winres.json`. Every release must raise it. The exe embeds `ASTROSPEED_VERSION=<v>;` and the in-app updater refuses a download that doesn't contain the exact version published. **Don't rename that marker.**
- **Tests:** `go test ./...` and `GOOS=windows go vet .` must pass. `catalog_test.go` enforces safety rules. For example, tweaks may **never touch the mouclass/kbdclass mouse and keyboard driver settings**, or sensitivity/pointer settings.
- **Servers:** Cloudflare Workers, built from `server/src/` by `python3 server/build_server.py`. That produces:
  - `server/astrospeed-server.js`, the full website worker (ROLE "all").
  - `server/key-hoster.js`, the key-only worker (ROLE "keys"). It only answers `/activate`, `/check`, `/health` and `/admin`, and its home page must just say **"Astro Tweaks key server is running."**
- **Storage:** both workers share one Cloudflare KV namespace bound as **`LICENSES`**. The free plan allows 1,000 writes a day (resets 8am Perth). Avoid extra KV writes.

## 3. Live setup (Cloudflare and Stripe)

| What | Where | Notes |
|---|---|---|
| Website, shop, downloads, updates, live tweaks, admin | Worker **`astrospeed`** → **https://astrotweaks.app** (custom domain), also `astrospeed.dupelication.workers.dev` | Paste `astrospeed-server.js` (we call it "WEBSITE - astrospeed.js") |
| Key hoster (app activates/checks keys here) | Worker **`rapid-forest-bfb5`** → `rapid-forest-bfb5.dupelication.workers.dev` | Paste `key-hoster.js` ("KEY HOSTER - rapid-forest-bfb5.js"). Must stay running. |
| Old exe host | Worker `astrospeedtweaks` | No longer needed; can be deleted |
| Admin page | **https://astrotweaks.app/admin** | Tabs: Keys, Sales, Release, Live tweaks, Settings |
| Health check | https://astrotweaks.app/health | Shows storage/password/stripe/webhook/stripeCharges |

**Worker variables and secrets (set in Cloudflare, never in chat):**
- Website worker: `LICENSES` (KV binding), `ADMIN_PASS` (secret, 12+ characters), `STRIPE_SECRET_KEY` (secret, live `sk_live_…`), `STRIPE_WEBHOOK_SECRET` (secret `whsec_…`), and `SITE_URL` = `https://astrotweaks.app` (text). `SITE_URL` redirects visitors from workers.dev to the domain; app routes are never redirected.
- Key hoster: `LICENSES` and `ADMIN_PASS`.

**Stripe:** live mode is working. Checkout uses Stripe-hosted Checkout with payment methods from the Stripe dashboard (Google Pay appears automatically on eligible devices).
- **Don't** send `payment_method_types`. Stripe's current API rejects it, and that broke checkout once.
- Webhook destination URL: `https://astrotweaks.app/stripe-webhook`. Events: `checkout.session.completed`, `charge.refunded`, `charge.dispute.created`. Payload: Snapshot.
- Sale keys are derived from the Stripe session ID, so one order always gives the same key.
- A refund or dispute disables the key automatically.
- Admin → Sales has a "Find missing orders" button.
- Price defaults to $20 (`price: 2000` cents). Settings saved before the change with the old 500 are treated as 2000 until saved again.

**Domain and email:** `astrotweaks.app` was bought on Cloudflare. **Cloudflare Email Routing** has a **catch-all** to the owner's Gmail, so any address @astrotweaks.app is received. It can't *send* from those addresses. The support email should be set to `support@astrotweaks.app` in admin → Settings.

**The app's built-in server addresses** (`server-url.txt`, embedded at build time):
```
https://astrospeed.dupelication.workers.dev
keys: https://rapid-forest-bfb5.dupelication.workers.dev
```
The app still uses the workers.dev addresses; they keep working, and the redirect handles browsers. Switching the app to astrotweaks.app is optional future work. Keep the old addresses running because existing installs use them.

## 4. Keys and licensing

- **Key format:** `ASTRO-XXXXX-XXXXX-XXXXX`. A key locks to the first PC that activates it (hashed MachineGuid). Admin can "Reset PC".
- **Offline:** the app keeps working for 14 days without reaching the server. Only clear server answers lock a key: disabled, other_pc, invalid or expired.
- **Trial keys:** created in admin with a number of days. When a trial ends, or the key is removed, the app **reverts the paid tweaks it applied** (`trial.json` mark, `needsRevert`).
- **Free version:** only 3 sample tweaks are free (`gamemode`, `gamedvr`, `telemetry`) plus the optimiser .bat. Everything else is paid.
  - Paid tweak names and details are **never sent to the window**. They're scrambled and blurred (`redact.go`).
  - Paid tools (booster, cleaner, startup manager, DNS, auto-boost, hotkey, memory guard, weekly clean, overlay, Fortnite tuner) are also blocked in Go (`gate.go` → `paidTool`).
  - Undo always works.
- **Upsell uses real numbers only:** e.g. "Your PC is only 35% optimized — 122 of 127 fixes locked". The owner once asked for a fake "your PC runs at 10%" claim. **We don't fabricate performance claims.**
- **Upgrade window:** pulls the live price from `/config.json`, appears at most once a day, and links to "My purchases".

## 5. Website (astrotweaks.app)

- White/purple design based on a reference layout the owner provided, using our own content and branding.
  - Drawn snowy mountains in SVG. The owner rejected a watermarked stock photo, so **don't use stock photos**.
  - Animations: kinetic headlines, falling snow, parallax, floating tiles, scrolling banner, and animated feature cards.
  - Dark-mode and pause-animation buttons.
- **Pages:** `/` (home), `/buy` (Stripe checkout), `/thanks` (shows the key), **`/purchases` (My purchases)**, `/download`, `/admin`, `/health`, `/config.json`, `/default-bat`, `/key-status`.
- **My purchases:** keys bought in a browser are saved in that browser's localStorage, so they're per PC, not on the server. The page shows a live status per key and lets people add a key manually.
- **Brand:**
  - Name: **Astro Tweaks**.
  - Colour: **purple** (`#7C3AED`, gradient `#6D28D9 → #8B5CF6 → #C084FC`). No orange any more.
  - Logo: the owner's white "A/T" mark, on a purple rounded square for the app icon.
  - Logo files are in `assets/` and `winres/`.

## 6. App features (as of 2.7.0)

- **Tweaks:** 150+ reversible tweaks in categories (Gaming, CPU, Power, Network, Graphics, Input, Apps, Privacy, Services, BIOS, Appearance, Online), plus per-game tweaks for Fortnite and others.
  - Every change is backed up and can be undone.
  - "Optimize" offers Recommended or Competitive.
  - Restore point first.
- **Live tweaks:** admin → Live tweaks adds .bat-based tweaks and a message banner, with no app update needed.
- **Optimiser .bat ("Fortnite optimiser"):**
  - Managed in **admin → Live tweaks → Optimiser .bat** (name, one-line "about", the .bat). No app update needed.
  - Everyone (free too) is asked: **"Allow Fortnite optimiser to make changes and open other apps?"**, with every command listed.
  - If allowed, the **whole .bat** runs (as admin, no console window) now and every time the app opens.
  - **It asks again every time the .bat changes.** This is deliberate security: a leaked admin password must not be able to silently push code to every PC.
  - People who said no aren't nagged. There's a switch at the bottom of Settings. Turning it off reverts the registry/service parts it changed.
  - The owner updates the .bat often and wants to do it without Claude. That's why it lives in admin.
  - **The owner's .bat hasn't been put into admin yet**, unless they did it themselves. The repo also has a `Fortniteopt.bat` they added.
- **PC Checkup:** score out of 100 with fix buttons. "Fix with a key" appears on locked items.
- **Benchmark page:**
  - Live FPS, 1% and 0.1% lows, and frame times, counted from Windows ETW DXGI Present events like PresentMon. Nothing is injected into games. DirectX games only (not Vulkan/OpenGL).
  - Also shows CPU, GPU usage, GPU temp/clock/power (NVIDIA via nvml.dll), VRAM, RAM, and CPU temp when Windows reports a real sensor.
  - Records "before" and "after" runs and makes a share image in Pictures\Astro Tweaks.
- **In-game overlay (paid), Afterburner-style:**
  - FPS, 1% low, frame time, a GPU row (usage, temp, clock, power), VRAM, CPU (usage, temp), RAM, and a frame-time graph.
  - Temperatures are colour-coded.
  - Corner, size and rows can be chosen, with a live preview in the app.
  - **Ctrl+Shift+O** hides or shows it.
  - It's a separate click-through topmost window, so it's anti-cheat safe. It works over borderless or windowed fullscreen only.
- **Fortnite settings tuner (paid):**
  - Edits Fortnite's GameUserSettings.ini: Max FPS or Balanced presets, Performance mode, and an FPS cap.
  - Fortnite must be closed. The file is backed up first and can be undone.
  - Never changes sensitivity or keybinds.
- **Input delay page:**
  - "Low delay mode" applies about 21 existing latency tweaks.
  - Mouse polling-rate tester.
  - Checklist of settings only the player can change (Reflex On + Boost, Performance mode, refresh rate, wired connections).
- **Other features:**
  - Fixes tweaks after Windows updates ("Windows changed back N tweaks", with an auto-fix option for key holders).
  - Game booster, cleaner, startup manager, DNS switcher, live ping graph.
  - Auto-boost, boost hotkey Ctrl+Alt+B.
  - Memory guard and weekly auto-clean.
  - In-app updater with a progress bar.
  - "What's new" after updates.

**Not yet tested on a real Windows PC**, so test these first:
- ETW FPS counter
- In-game overlay
- Fortnite tuner
- Polling-rate test (headless testing could only fake about 60 Hz)
- CPU temperature reading
- Optimiser .bat actually running

## 7. How to release an update

1. Raise `AppVersion` in `main.go` and the version in `winres/winres.json`.
2. Build the exe (`build.bat` on Windows).
3. Go to https://astrotweaks.app/admin → **Release** and pick `Astro Tweaks.exe`. The version fills in by itself. Write notes, then click **Upload and publish**.
4. Every app shows "Update now", then "What's new". The website's Download button gives out the new exe.
5. If server code changed: rebuild with `python3 server/build_server.py`, then paste `astrospeed-server.js` into the **astrospeed** worker and `key-hoster.js` into **rapid-forest-bfb5**.

## 8. Rules we agreed on (keep following these)

- **Honest marketing:** no fake numbers, fake reviews, or claims the app can't back up.
- **Anti-cheat safety:** never inject into games, edit game memory or files, or install kernel drivers. Editing Fortnite's *settings file* is fine. The site says "never touches the game itself".
- **Every change must be reversible and backed up.** Undo must always work, even for free users.
- **Never touch** mouse/keyboard driver settings (mouclass/kbdclass), sensitivity, DPI or pointer settings.
- **Consent for running other programs:** anything that runs arbitrary commands or opens or installs other apps needs a clear opt-in, and must ask again when it changes. No silent installs (they're treated as unwanted software).
- **The key-hoster home page** stays plain text. Its admin page is a duplicate of the main one; the main admin is astrotweaks.app/admin.
- **Don't mention GitHub** on the website or in the app.
- **Secrets** (Stripe keys, admin password, webhook secret) only ever go into Cloudflare, never into chat.
- Keep the old workers.dev addresses running, because existing installs use them.

## 9. Ideas not done yet (the owner may ask)

- Discount and creator codes through Stripe promotion codes (`allow_promotion_codes`).
- A self-serve free-trial button on the website (one per PC).
- Sale alerts posted to a Discord channel.
- A tray icon so auto-boost, overlay and memory guard work with the window closed.
- Self-service key transfer to a new PC.
- Pointing the app's built-in addresses at astrotweaks.app.
- Sending email *as* support@astrotweaks.app (needs Zoho or Google Workspace, or an SMTP service).

## 10. Docs in the repo

`START HERE.txt` (setup), `HOW TO RELEASE AN UPDATE.txt`, `HOW TO ADD YOUR BAT.txt`, `README.md`, and this file.
