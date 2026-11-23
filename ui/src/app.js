(() => {
'use strict';

// ───────────────────────── bridge to the Go backend ─────────────────────────
const pending = new Map();
const listeners = {};
let seq = 0;
window.__sg = {
  done(id, payload) {
    const p = pending.get(id);
    if (!p) return;
    pending.delete(id);
    if (payload && payload.error !== undefined) p.reject(new Error(payload.error));
    else p.resolve(payload ? payload.ok : null);
  },
  event(name, data) { (listeners[name] || []).slice().forEach(f => f(data)); }
};
const on = (name, f) => { (listeners[name] = listeners[name] || []).push(f); return () => { listeners[name] = listeners[name].filter(x => x !== f); }; };
const api = (method, args) => new Promise((resolve, reject) => {
  const id = ++seq;
  pending.set(id, { resolve, reject });
  const fn = window.sgInvoke || window.__sgMock;
  try { fn(method, JSON.stringify(args === undefined ? null : args), id); }
  catch (e) { pending.delete(id); reject(e); }
});

// ───────────────────────── helpers ─────────────────────────
const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => Array.from(el.querySelectorAll(s));
const esc = s => String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const fmtBytes = b => { if (!b) return '0 MB'; const mb = b / 1048576; return mb >= 1024 ? (mb / 1024).toFixed(1) + ' GB' : mb >= 10 ? Math.round(mb) + ' MB' : mb.toFixed(1) + ' MB'; };
const fmtMB = mb => mb >= 1024 ? (mb / 1024).toFixed(1) + ' GB' : Math.round(mb) + ' MB';
const fmtDate = d => { const t = new Date(d); if (!d || isNaN(t) || t.getFullYear() < 2000) return ''; return t.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit' }); };
const plural = (n, a, b) => n + ' ' + (n === 1 ? a : (b || a + 's'));

const P = {
  dashboard: '<rect x="3.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="3.5" width="7" height="4.5" rx="1.5"/><rect x="13.5" y="11" width="7" height="9.5" rx="1.5"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.5"/>',
  fortnite: '<circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="12" r="4"/><path d="M12 1.5v4M12 18.5v4M1.5 12h4M18.5 12h4"/>',
  gamepad: '<path d="M7 8h10a4.5 4.5 0 0 1 4.4 5.5l-.8 3.6a2.4 2.4 0 0 1-4.2 1L14.6 16H9.4l-1.8 2.1a2.4 2.4 0 0 1-4.2-1l-.8-3.6A4.5 4.5 0 0 1 7 8Z"/><path d="M8 11v3M6.5 12.5h3M15.5 12h.01M17.5 13.5h.01"/>',
  cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx=".8"/><path d="M9 2.5V6M15 2.5V6M9 18v3.5M15 18v3.5M2.5 9H6M2.5 15H6M18 9h3.5M18 15h3.5"/>',
  bolt: '<path d="M13 2.5 4.5 13.5H12l-1 8 8.5-11H12z"/>',
  wifi: '<path d="M2.5 9a14 14 0 0 1 19 0M5.5 12.5a9.5 9.5 0 0 1 13 0M8.6 16a5 5 0 0 1 6.8 0"/><circle cx="12" cy="19.5" r=".9"/>',
  gpu: '<rect x="2.5" y="6" width="19" height="11" rx="2"/><circle cx="9" cy="11.5" r="3"/><path d="M15 9.5h3.5M15 13h3.5M5 17v2.5M10 17v2.5"/>',
  mouse: '<rect x="6" y="2.5" width="12" height="19" rx="6"/><path d="M12 6.5v3.5"/>',
  chat: '<path d="M4 5.5h16a1.5 1.5 0 0 1 1.5 1.5v9a1.5 1.5 0 0 1-1.5 1.5h-8L7 21v-3.5H4A1.5 1.5 0 0 1 2.5 16V7A1.5 1.5 0 0 1 4 5.5Z"/><path d="M8 10.5h.01M12 10.5h.01M16 10.5h.01"/>',
  eye: '<path d="M2 12s3.6-6.5 10-6.5S22 12 22 12s-3.6 6.5-10 6.5S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/>',
  layers: '<path d="m12 3 9.5 5-9.5 5-9.5-5z"/><path d="m2.5 12.5 9.5 5 9.5-5"/><path d="m2.5 16.5 9.5 5 9.5-5"/>',
  chip: '<rect x="4" y="4" width="16" height="16" rx="2.5"/><path d="M8 8h3v3H8zM13 13h3v3h-3zM13 8h3M8 16h3"/>',
  palette: '<path d="M12 2.5a9.5 9.5 0 1 0 0 19c1.4 0 2-1 1.6-2.2-.5-1.4.4-2.8 1.9-2.8h2.2a3.8 3.8 0 0 0 3.8-3.8C21.5 6.6 17.2 2.5 12 2.5Z"/><circle cx="7.5" cy="11" r="1.2"/><circle cx="10.5" cy="7" r="1.2"/><circle cx="15.5" cy="7.5" r="1.2"/>',
  rocket: '<path d="M14.5 4.5c2.8-1.6 5.4-1.8 6.5-1.5.3 1.1.1 3.7-1.5 6.5L13 16.5 7.5 11z"/><path d="M7.5 11 4 10.5l3-3.5 4 .5M13 16.5l.5 3.5 3.5-3-.5-4"/><path d="M6 15.5c-1.5.5-2.5 2.5-2.5 5 2.5 0 4.5-1 5-2.5"/>',
  broom: '<path d="m14 3 7 7M17.5 6.5 11 13"/><path d="M11 13c-3-1-6 .5-7.5 4.5L8 21.5c3.5-1 5.5-4 5-7.5z"/><path d="m6.5 16.5 2.5 2.5"/>',
  power: '<path d="M12 3v8"/><path d="M6.4 6.5a8 8 0 1 0 11.2 0"/>',
  signal: '<path d="M4 20v-3M9 20v-7M14 20V9M19 20V4"/>',
  shield: '<path d="M12 2.5 4 5.5v6c0 4.8 3.4 8.8 8 10 4.6-1.2 8-5.2 8-10v-6z"/><path d="m8.5 12 2.5 2.5 4.5-5"/>',
  info: '<circle cx="12" cy="12" r="9.5"/><path d="M12 11v5.5M12 7.5h.01"/>',
  check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
  ok: '<circle cx="12" cy="12" r="9.5"/><path d="m7.8 12.3 2.8 2.8 5.6-5.8"/>',
  warn: '<path d="M10.3 3.9 2.4 18a2 2 0 0 0 1.7 3h15.8a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z"/><path d="M12 9.5v4.5M12 17.5h.01"/>',
  bad: '<circle cx="12" cy="12" r="9.5"/><path d="m9 9 6 6M15 9l-6 6"/>',
  dash: '<circle cx="12" cy="12" r="9.5"/><path d="M8 12h8"/>',
  x: '<path d="m6 6 12 12M18 6 6 18"/>',
  reboot: '<path d="M20 12a8 8 0 1 1-2.3-5.6"/><path d="M20 4v4.5h-4.5"/>',
  undo: '<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>',
  spark: '<path d="M12 3v4M12 17v4M3 12h4M17 12h4M6 6l2.5 2.5M15.5 15.5 18 18M6 18l2.5-2.5M15.5 8.5 18 6"/>',
  lock: '<rect x="4.5" y="10.5" width="15" height="10" rx="2"/><path d="M8 10.5V7.5a4 4 0 0 1 8 0v3"/>',
  arrow: '<path d="M5 12h14M13 6l6 6-6 6"/>',
  ext: '<path d="M14 4h6v6M20 4l-9 9"/><path d="M18 14v5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 4 19V7.5A1.5 1.5 0 0 1 5.5 6H10"/>',
  log: '<path d="M6 3.5h9l4 4v13H6z"/><path d="M14.5 3.5V8H19M9 12h7M9 15.5h7M9 19h4"/>',
  glider: '<path d="M21 3 3 10.5l7.5 2.5L13 21z"/><path d="m10.5 13 4-4"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
  moon: '<path d="M21 13A9 9 0 0 1 11 3a9 9 0 1 0 10 10Z"/>',
  pause: '<path d="M8 5v14M16 5v14"/>',
  gear: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 0 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 0 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 0 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 0 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z"/>',
  pulse: '<path d="M2.5 12h4l2.5-6 4.5 12 2.5-6h5.5"/>',
  monitor: '<rect x="2.5" y="4" width="19" height="13" rx="2"/><path d="M8.5 21h7M12 17v4"/>',
  clock: '<circle cx="12" cy="12" r="9.5"/><path d="M12 7v5l3.5 2"/>',
  play: '<path d="M7 4.5v15l12-7.5z"/>'
};
const ico = (n, cls) => `<svg viewBox="0 0 24 24" aria-hidden="true"${cls ? ` class="${cls}"` : ''}>${P[n] || ''}</svg>`;

// ───────────────────────── app state ─────────────────────────
const S = {
  data: null, states: {}, reboot: false, page: 'dashboard', filter: 'all', search: '',
  expanded: new Set(), busy: new Set(), heroAnimated: false,
  booster: null, bgApps: null, bgSel: null, clean: null, cleanSel: null, startup: null,
  ping: null, pinging: false, dns: null, log: null,
  checkup: null, scanning: false, scanStamp: 0, livePing: { region: 'oce', running: false, samples: [] }
};
const T = () => S.data.catalog;
const byId = id => T().find(t => t.id === id);
const st = id => (S.states[id] || {}).s || 'off';
const isOn = id => ['on', 'native'].includes(st(id));
const avail = id => st(id) !== 'na';
const catTweaks = c => T().filter(t => t.cat === c);
const appliedCount = list => list.filter(t => isOn(t.id)).length;
const availCount = list => list.filter(t => avail(t.id)).length;
const pendingFor = p => T().filter(t => t.profile > 0 && t.profile <= p && avail(t.id) && !isOn(t.id));
const LIC = () => (S.data && S.data.license) || { required: false, activated: true };
const proUnlocked = () => { const l = LIC(); return !l.required || l.activated; };
const locked = t => !!t.pro && !proUnlocked();
const BUY_URL = 'https://astrospeed.dupelication.workers.dev/';

const toolPages = [
  { id: 'checkup', title: 'PC checkup', icon: 'pulse', sub: 'A full scan of what is holding your FPS back, with a score and one-click fixes.' },
  { id: 'booster', title: 'Game booster', icon: 'rocket', sub: 'One-tap actions to run right before you queue: clear memory, tighten the timer, close background apps.' },
  { id: 'cleaner', title: 'System cleaner', icon: 'broom', sub: 'Clears temp files, caches and leftovers. Your game settings, keybinds and replays are never touched.' },
  { id: 'startup', title: 'Startup apps', icon: 'power', sub: 'Choose what launches with Windows. Uses the same switch as Task Manager, so you can change it back there too.' },
  { id: 'network', title: 'Ping & DNS', icon: 'signal', sub: 'Test your ping to every Fortnite region and pick a DNS server.' }
];
const safetyPages = [
  { id: 'settings', title: 'Settings', icon: 'gear', sub: 'Appearance, updates, the boost hotkey and starting with Windows.' },
  { id: 'safety', title: 'Restore & undo', icon: 'shield', sub: 'Every change AstroSpeed makes is backed up first. Undo one tweak, a section, or everything.' },
  { id: 'about', title: 'About', icon: 'info', sub: 'What AstroSpeed changes, what it never touches, and where your backups live.' }
];

// ───────────────────────── navigation ─────────────────────────
function renderNav() {
  const cats = S.data.categories.filter(c => !c.game).map(c => {
    const list = catTweaks(c.id);
    const a = appliedCount(list), n = availCount(list);
    return navItem('cat:' + c.id, c.nav || c.title, c.icon, `<span class="count"><b>${a}</b>/${n}</span>`, n > 0 && a === n);
  }).join('');
  const gf = gamesFound().length;
  $('#nav').innerHTML = `
    <div class="nav-group">${navItem('dashboard', 'Dashboard', 'dashboard')}${navItem('games', 'Games', 'gamepad', gf ? `<span class="count"><b>${gf}</b> found</span>` : '')}</div>
    <div class="nav-group"><div class="nav-label">Tweaks</div>${cats}</div>
    <div class="nav-group"><div class="nav-label">Tools</div>${toolPages.map(p => navItem(p.id, p.title, p.icon)).join('')}</div>
    <div class="nav-group"><div class="nav-label">App</div>${safetyPages.map(p => navItem(p.id, p.title, p.icon)).join('')}</div>`;
  const e = S.data.env || {};
  $('#railFoot').innerHTML = `
    <div class="foot-row"><span class="dot ${S.data.admin ? 'on' : 'off'}"></span>${S.data.admin ? 'Running as administrator' : 'Not running as administrator'}</div>
    <div class="foot-row"><span class="dot ${gf ? 'on' : 'warn'}"></span>${gf ? plural(gf, 'game') + ' found' : 'No games found yet'}</div>
    ${licenseChip()}
    ${S.update && S.update.available ? `<button class="upd-pill" data-act="showUpdate" type="button"><i></i>Update ${esc(S.update.latest)} available</button>` : `<div class="foot-row dim">Version ${esc(S.data.version)}</div>`}`;
  moveNavIndicator();
}
function navItem(page, title, icon, extra = '', full = false) {
  const active = S.page === page || (page === 'games' && isGameCat(S.page));
  return `<button class="nav-item${active ? ' active' : ''}${full ? ' full' : ''}" data-nav="${page}" type="button" title="${esc(title)}">${ico(icon)}<span class="nl">${esc(title)}</span>${extra}</button>`;
}
let navPlaced = false;
function moveNavIndicator() {
  const ind = $('#navInd'), act = $('#nav .nav-item.active');
  if (!ind) return;
  if (!act) { ind.style.opacity = '0'; return; }
  if (!navPlaced) { ind.style.transition = 'none'; }
  ind.style.transform = `translateY(${act.offsetTop}px)`;
  ind.style.height = act.offsetHeight + 'px';
  ind.style.opacity = '1';
  if (!navPlaced) { void ind.offsetWidth; ind.style.transition = ''; navPlaced = true; }
}
function go(page) {
  if (page === S.page && page !== 'search') { $('#view').scrollTo({ top: 0, behavior: 'smooth' }); return; }
  S.page = page;
  if (page !== 'search') { S.search = ''; $('#search').value = ''; }
  S.filter = 'all';
  S.anim = true;
  if (page === 'cat:kernel' && !S.bcdReady) { S.bcdReady = true; refreshStates(); }
  render();
  $('#view').scrollTop = 0;
}

// ───────────────────────── top-level render ─────────────────────────
function render() {
  const animate = !!S.anim;
  renderNav();
  renderBanners();
  const v = $('#view');
  v.classList.remove('enter');
  let title = '', sub = '';
  if (S.page === 'dashboard') { title = 'Dashboard'; sub = 'Your PC at a glance, and how much of AstroSpeed is switched on.'; v.innerHTML = viewDashboard(); }
  else if (S.page.startsWith('cat:')) {
    const c = S.data.categories.find(x => x.id === S.page.slice(4));
    title = c.title; sub = c.subtitle; v.innerHTML = viewCategory(c);
  }
  else if (S.page === 'search') { title = 'Search'; sub = `Tweaks matching “${S.search}”`; v.innerHTML = viewSearch(); }
  else if (S.page === 'games') { title = 'Games'; sub = 'Tweaks for each supported game on this PC. Each one only changes how Windows treats that game.'; v.innerHTML = viewGames(); }
  else {
    const p = [...toolPages, ...safetyPages].find(x => x.id === S.page);
    title = p.title; sub = p.sub;
    v.innerHTML = ({ checkup: viewCheckup, booster: viewBooster, cleaner: viewCleaner, startup: viewStartup, network: viewNetwork, safety: viewSafety, about: viewAbout, settings: viewSettings })[S.page]();
    afterTool();
  }
  const tb = $('#titleBlock');
  if (tb.dataset.title !== title || S.anim) {
    tb.dataset.title = title;
    $('#pageTitle').innerHTML = kinetic(title);
    $('#pageSub').textContent = sub;
    tb.classList.remove('swap'); void tb.offsetWidth; tb.classList.add('swap');
  } else { $('#pageSub').textContent = sub; }
  if (S.anim) S.anim = false;
  if (S.page === 'dashboard') { afterDashboard(animate); if (!liveTimer) startLive(); } else stopLive();
  if (S.splashGone) setupReveal(animate);
  updateScrollBar();
  if (S.flash) { const f = S.flash; S.flash = null; const row = $(`[data-row="${f.id}"]`); if (row) { row.classList.add('flash'); if (!f.on) row.classList.add('off'); } }
}

function renderBanners() {
  let h = '';
  if (!S.data.admin) h += `<div class="banner warn">${ico('warn')}<span>AstroSpeed isn't running as administrator, so most tweaks can't be applied. Close it, then right-click it and choose <b>Run as administrator</b>.</span></div>`;
  if (S.data.lastCrash && !S.crashSeen) h += `<div class="banner crash">${ico('info')}<span>AstroSpeed didn't close properly last time. Your settings and backups are safe.</span><button class="btn sm ghost" data-act="crashDetails" type="button">Details</button><button class="btn sm quiet" data-act="crashDismiss" type="button" style="margin-left:0">Dismiss</button></div>`;
  const pk = S.data.pack || {};
  if (pk.message && prefGet('sg-msg-seen') !== pk.message) h += `<div class="banner news">${ico('spark')}<span>${esc(pk.message)}</span><button class="btn sm quiet" data-act="msgDismiss" type="button" style="margin-left:auto">Dismiss</button></div>`;
  if (S.reboot) h += `<div class="banner reboot">${ico('reboot')}<span>Restart your PC to finish applying some changes.</span><button class="btn sm primary" data-act="restart" type="button">Restart now</button></div>`;
  const b = $('#banners');
  if (b.innerHTML !== h) b.innerHTML = h;
}

// ───────────────────────── dashboard ─────────────────────────
function dashNumbers() {
  const auto = T().filter(t => t.profile > 0 && avail(t.id));
  const pct = auto.length ? Math.round(appliedCount(auto) / auto.length * 100) : 0;
  return { auto, pct, frac: auto.length ? appliedCount(auto) / auto.length : 0 };
}

function viewDashboard() {
  const { pct } = dashNumbers();
  const pendRec = pendingFor(1).length, pendComp = pendingFor(2).length;
  const allOn = appliedCount(T());
  const e = S.data.env || {};
  const first = !S.heroDone;
  let line;
  const full = pct >= 98;
  if (full) line = 'Everything in Recommended and Competitive is switched on. Manual-only extras are waiting in each section if you want them.';
  else line = pct >= 60
    ? `${plural(pendComp, 'tweak')} in the Competitive profile ${pendComp === 1 ? 'is' : 'are'} still off. Optimize switches them on after making a restore point.`
    : `${plural(pendRec, 'safe tweak')} and ${pendComp - pendRec} competitive ones are still off. Optimize applies them in one go, after making a restore point, and every change can be undone.`;
  const headline = full
    ? kinetic('Fully optimised.') + '<br>' + kinetic('Ready to drop.', 2)
    : `<span class="kw"><span><span class="pct" id="pctNum">${first ? 0 : pct}</span>%</span></span> ` + kinetic('optimised', 1) + '<br>' + kinetic('for gaming.', 2);
  const nf = gamesFound().length;
  const kicker = nf ? `${plural(nf, 'game')} found on this PC` : 'No supported games found yet. System tweaks still apply.';
  const cnt = v => first ? `<span data-count="${v}">0</span>` : v;
  const last = S.data.lastOptimize && fmtDate(S.data.lastOptimize);
  const tiles = [
    ['spark', 'FPS', 'Frame pacing'],
    ['signal', 'PING', 'Network'],
    ['lock', 'SENS', 'Untouched'],
    ['undo', 'UNDO', 'Any time']
  ];
  return `
  <section class="hero" id="hero">
    <div class="hero-land" id="heroLand" aria-hidden="true">
      <img class="land day" alt="" src="${window.__HILLS_DAY || ''}" draggable="false">
      <img class="land night" alt="" src="${window.__HILLS_NIGHT || ''}" draggable="false">
    </div>
    <svg class="contours" id="contours" aria-hidden="true"></svg>
    <svg class="flightpath" id="flight" aria-hidden="true"></svg>
    <div class="glider" id="glider" aria-hidden="true"><div class="bob">${gliderSVG()}</div></div>
    <div class="depth" id="depth" aria-hidden="true">
      ${tiles.map((t, i) => `<div class="tile t${i}"><div class="tile-in">${ico(t[0])}<b>${t[1]}</b><small>${t[2]}</small></div></div>`).join('')}
    </div>
    <div class="hero-copy">
      <p class="chip hero-step" style="--s:0"><span class="live"></span>${esc(kicker)}</p>
      <h2 class="kinetic-h">${headline}</h2>
      <p class="lede hero-step" style="--s:4">${esc(line)}</p>
      <div class="hero-actions hero-step" style="--s:5">
        <button class="btn primary lg" data-act="optimize" type="button">${ico('glider')}<span>Optimize now</span>${ico('arrow', 'arr')}</button>
        <button class="btn secondary lg" data-nav="safety" type="button">${ico('shield')}Restore &amp; undo</button>
      </div>
      <p class="compat hero-step" style="--s:6"><b>${cnt(allOn)}</b> tweaks on<i></i><b>${cnt(S.data.totalChanges || 0)}</b> changes, all reversible<i></i>${last ? 'Last optimized ' + esc(last) : 'Restore point made first'}</p>
    </div>
  </section>
  <div class="ribbon" aria-hidden="true"><div class="ribbon-track">${ribbonWords()}${ribbonWords()}</div></div>
  <div class="dash-row">${liveHTML()}${yourGamesHTML()}</div>
  <div class="dash-grid">
    <div class="panel spot"><div class="panel-h"><h2>This PC</h2><button class="btn sm quiet" data-act="reprobe" type="button">${ico('reboot')}Rescan</button></div><div class="panel-b">${specList()}</div></div>
    <div class="panel spot"><div class="panel-h"><h2>Health checks</h2></div><div class="panel-b"><div class="checks">${healthChecks()}</div></div></div>
    <div class="panel spot"><div class="panel-h"><h2>Never changed</h2></div><div class="panel-b protect">
      <p class="muted small" style="margin:0 0 8px">No tweak in AstroSpeed touches any of these:</p>
      <ul>${['Mouse sensitivity, pointer speed and “Enhance pointer precision”', 'Mouse DPI, polling rate and your mouse software', 'Game settings, keybinds, crosshair and replays', 'Epic Games Launcher settings', 'Screen resolution, refresh rate and scaling', 'Keyboard repeat rate and layout', 'Windows Defender, Firewall, UAC and Windows Update']
        .map(x => `<li>${ico('lock')}<span>${esc(x)}</span></li>`).join('')}</ul>
    </div></div>
  </div>`;
}

function ribbonWords() {
  const star = '<i><svg viewBox="0 0 24 24"><path d="M12 1v22M1 12h22M4 4l16 16M20 4 4 20"/></svg></i>';
  return ['OPTIMIZE', 'LOWER LATENCY', 'STEADIER FRAMES', 'FULLY REVERSIBLE', 'YOUR SETTINGS UNTOUCHED'].map(w => `<span>${w}</span>${star}`).join('');
}

// wraps each word for the word-by-word reveal; `from` offsets the stagger
function kinetic(text, from = 0) {
  return String(text).split(/\s+/).filter(Boolean).map((w, i) => `<span class="kw" style="--k:${from + i}"><span>${esc(w)}</span></span>`).join(' ');
}

function gliderSVG() {
  return `<svg viewBox="0 0 84 28"><defs><linearGradient id="ctail" x1="0" x2="1"><stop offset="0" stop-color="var(--accent)" stop-opacity="0"/><stop offset="1" stop-color="var(--accent)" stop-opacity=".85"/></linearGradient><radialGradient id="chead" cx=".4" cy=".4" r=".6"><stop offset="0" stop-color="#fff"/><stop offset=".5" stop-color="#FFD9C2"/><stop offset="1" stop-color="var(--accent)"/></radialGradient></defs><path d="M2 14 L70 6.5 L70 21.5 Z" fill="url(#ctail)"/><circle cx="70" cy="14" r="13" fill="var(--accent)" opacity=".18"/><circle cx="70" cy="14" r="7.5" fill="url(#chead)"/></svg>`;
}

const easeOut = t => 1 - Math.pow(1 - t, 4);
const easeInOut = t => t < .5 ? 8 * t * t * t * t : 1 - Math.pow(-2 * t + 2, 4) / 2;
const motionOK = () => !document.documentElement.classList.contains('motion-off') && !matchMedia('(prefers-reduced-motion: reduce)').matches;
function tween(dur, fn, done, ease = easeOut) {
  const t0 = performance.now();
  const step = now => {
    const t = Math.min(1, (now - t0) / dur);
    fn(ease(t));
    if (t < 1) requestAnimationFrame(step); else if (done) done();
  };
  requestAnimationFrame(step);
}
function countUp(el, to, dur = 1600, delay = 0) {
  if (!motionOK()) { el.textContent = to; return; }
  setTimeout(() => tween(dur, k => { el.textContent = Math.round(to * k); }), delay);
}

// topographic contour lines over the landscape, drawn once per size
function drawContours() {
  const svg = $('#contours'), hero = $('#hero');
  if (!svg || !hero) return;
  const W = hero.clientWidth, H = hero.clientHeight;
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  let rnd = 7; const r = () => (rnd = (rnd * 16807) % 2147483647) / 2147483647;
  const centres = [[W * 0.78, H * 0.34, 1], [W * 0.18, H * 0.9, 0.8], [W * 0.98, H * 0.95, 0.7]];
  let d = '';
  for (const [cx, cy, sc] of centres) {
    const ph = [r() * 6.28, r() * 6.28, r() * 6.28];
    for (let ring = 1; ring <= 9; ring++) {
      const R = ring * 46 * sc;
      let p = '';
      for (let a = 0; a <= 64; a++) {
        const t = a / 64 * Math.PI * 2;
        const k = 1 + 0.16 * Math.sin(3 * t + ph[0] + ring * 0.3) + 0.08 * Math.sin(5 * t + ph[1]) + 0.05 * Math.sin(7 * t + ph[2] - ring * 0.2);
        const x = cx + Math.cos(t) * R * k * 1.35, y = cy + Math.sin(t) * R * k * 0.8;
        p += (a ? 'L' : 'M') + x.toFixed(1) + ' ' + y.toFixed(1);
      }
      d += p + 'Z';
    }
  }
  svg.innerHTML = `<path d="${d}"/>`;
}

// the glider's flight path across the hero; progress = how optimised the PC is
function flightGeo() {
  const hero = $('#hero');
  if (!hero) return null;
  const W = hero.clientWidth, H = hero.clientHeight;
  const x0 = W * 0.43, y0 = H * 1.02, x1 = W * 0.955, y1 = H * 0.15;
  return { W, H, d: `M ${x0} ${y0} C ${W * 0.62} ${H * 0.96}, ${W * 0.72} ${H * 0.34}, ${x1} ${y1}` };
}
function setupFlight() {
  const svg = $('#flight'), G = flightGeo();
  if (!svg || !G) return null;
  svg.setAttribute('viewBox', `0 0 ${G.W} ${G.H}`);
  svg.innerHTML = `<path class="route" d="${G.d}"/><path class="done" id="flightDone" d="${G.d}"/><circle class="goal" r="5"/>`;
  const path = $('#flightDone'), len = path.getTotalLength();
  const end = path.getPointAtLength(len);
  const goal = svg.querySelector('.goal');
  goal.setAttribute('cx', end.x); goal.setAttribute('cy', end.y);
  path.style.strokeDasharray = `${len} ${len}`;
  return { path, len };
}
function placeGliderAt(F, frac) {
  const gl = $('#glider');
  if (!F || !gl) return;
  const L = Math.max(0.001, Math.min(1, frac)) * F.len;
  const p = F.path.getPointAtLength(L), q = F.path.getPointAtLength(Math.min(F.len, L + 2));
  const ang = Math.atan2(q.y - p.y, q.x - p.x) * 180 / Math.PI;
  gl.style.transform = `translate(${p.x}px, ${p.y}px) rotate(${ang}deg)`;
  F.path.style.strokeDashoffset = String(F.len - L);
}

function afterDashboard(animate) {
  const hero = $('#hero');
  if (!hero) return;
  if (!animate && S.heroDone) hero.classList.add('still');
  drawContours();
  const F = setupFlight();
  const { frac, pct } = dashNumbers();
  if (!S.splashGone) { placeGliderAt(F, 0); return; }
  requestAnimationFrame(() => hero.classList.add('play'));
  if (!S.heroDone && motionOK()) {
    S.heroDone = true;
    placeGliderAt(F, 0.001);
    setTimeout(() => tween(2800, k => placeGliderAt(F, frac * k), null, easeInOut), 500);
    const p = $('#pctNum');
    if (p) { p.textContent = pct; p.style.minWidth = p.getBoundingClientRect().width + 'px'; p.textContent = '0'; countUp(p, pct, 2800, 500); }
    $$('[data-count]', hero).forEach((el, i) => countUp(el, +el.dataset.count, 1800, 900 + i * 140));
  } else {
    S.heroDone = true;
    const from = S.lastFrac != null ? Math.min(S.lastFrac, frac) : frac;
    if (Math.abs(from - frac) > 0.005 && motionOK()) tween(1500, k => placeGliderAt(F, from + (frac - from) * k), null, easeInOut);
    else placeGliderAt(F, frac);
  }
  S.lastFrac = frac;
}

// pointer: spotlight on cards, depth and parallax in the hero
let parX = 0, parY = 0, parTX = 0, parTY = 0, parRAF = 0;
function parallaxLoop() {
  parX += (parTX - parX) * 0.06; parY += (parTY - parY) * 0.06;
  const land = $('#heroLand'), depth = $('#depth'), cont = $('#contours');
  if (land) land.style.transform = `translate3d(${parX * -14}px, ${parY * -8}px, 0) scale(1.04)`;
  if (cont) cont.style.transform = `translate3d(${parX * -24}px, ${parY * -14}px, 0)`;
  if (depth) depth.style.transform = `rotateY(${parX * 9}deg) rotateX(${parY * -7}deg)`;
  if (Math.abs(parTX - parX) > 0.001 || Math.abs(parTY - parY) > 0.001) parRAF = requestAnimationFrame(parallaxLoop);
  else parRAF = 0;
}
document.addEventListener('pointermove', e => {
  const sp = e.target.closest && e.target.closest('.spot');
  if (sp) { const r = sp.getBoundingClientRect(); sp.style.setProperty('--mx', (e.clientX - r.left) + 'px'); sp.style.setProperty('--my', (e.clientY - r.top) + 'px'); }
  if (!motionOK()) return;
  const hero = e.target.closest && e.target.closest('#hero');
  if (hero) {
    const r = hero.getBoundingClientRect();
    parTX = (e.clientX - r.left) / r.width - 0.5; parTY = (e.clientY - r.top) / r.height - 0.5;
  } else { parTX = 0; parTY = 0; }
  if (!parRAF) parRAF = requestAnimationFrame(parallaxLoop);
}, { passive: true });

// cards and groups tilt up into place as they scroll into view
let revealIO = null;
function setupReveal(animate) {
  const view = $('#view');
  if (revealIO) revealIO.disconnect();
  const els = $$('#view .panel:not(.hero-panel), #view .group, #view .toolbar, #view .empty, #view .rows.single > .row, #view > .grid > *');
  if (!motionOK() || !animate) { els.forEach(el => el.classList.add('rv', 'in')); return; }
  let batch = 0, batchT = 0;
  revealIO = new IntersectionObserver(entries => {
    const now = performance.now();
    if (now - batchT > 120) batch = 0;
    batchT = now;
    entries.filter(en => en.isIntersecting).forEach(en => {
      en.target.style.setProperty('--rd', (batch++ * 70) + 'ms');
      en.target.classList.add('in');
      revealIO.unobserve(en.target);
    });
  }, { root: view, threshold: 0.08, rootMargin: '0px 0px -4% 0px' });
  els.forEach((el, i) => {
    if (el.classList.contains('in')) return;
    el.classList.add('rv', i % 2 ? 'rv-r' : 'rv-l');
    revealIO.observe(el);
  });
}

// thin progress line under the top bar while scrolling a long page
function updateScrollBar() {
  const v = $('#view'), bar = $('#scrollBar');
  if (!v || !bar) return;
  const max = v.scrollHeight - v.clientHeight;
  bar.style.transform = `scaleX(${max > 0 ? v.scrollTop / max : 0})`;
}

const prefGet = k => { try { return localStorage.getItem(k) || ''; } catch (e) { return ''; } };
const prefSet = (k, v) => { try { localStorage.setItem(k, v); } catch (e) { /* ignore */ } };
// appearance and motion preferences (kept in this browser profile only)
function loadPrefs() {
  let theme = 'light', motion = 'on';
  try { theme = localStorage.getItem('sg-theme') || 'light'; motion = localStorage.getItem('sg-motion') || 'on'; } catch (e) { /* storage unavailable */ }
  applyTheme(theme, false); applyMotion(motion === 'on');
}
function applyTheme(t, save = true) {
  document.documentElement.dataset.theme = t;
  const b = $('#themeBtn');
  if (b) { b.setAttribute('aria-label', t === 'dark' ? 'Use light appearance' : 'Use dark appearance'); b.innerHTML = ico(t === 'dark' ? 'sun' : 'moon'); }
  if (save) try { localStorage.setItem('sg-theme', t); } catch (e) { /* ignore */ }
}
function applyMotion(on, save = false) {
  document.documentElement.classList.toggle('motion-off', !on);
  const b = $('#motionBtn');
  if (b) { b.setAttribute('aria-pressed', String(!on)); b.setAttribute('aria-label', on ? 'Pause animations' : 'Play animations'); b.innerHTML = ico(on ? 'pause' : 'play'); }
  if (save) try { localStorage.setItem('sg-motion', on ? 'on' : 'off'); } catch (e) { /* ignore */ }
}

function specList() {
  const e = S.data.env || {};
  const gpus = (e.gpus || []).map(g => `${esc(g.name)}${g.vramGB >= 1 ? ` <span class="dim">${Math.round(g.vramGB)} GB</span>` : ''}`).join('<br>') || '<span class="dim">Unknown</span>';
  let ram = `${Math.round(e.ramGB || 0)} GB`;
  if (e.ramType) ram += ' ' + esc(e.ramType);
  if (e.ramConf) ram += ` at ${e.ramConf} MT/s`;
  let ramNote = '';
  if (e.ramXmp === 'likely_off') ramNote = `<span class="note" style="color:var(--amber)">Running at default speed. XMP/EXPO may be off in BIOS.</span>`;
  else if (e.ramXmp === 'below_rated') ramNote = `<span class="note" style="color:var(--amber)">Rated for ${e.ramSpeed} MT/s</span>`;
  const rows = [
    ['Windows', esc(e.osName || 'Windows') + (e.osVersion ? `<span class="note dim">${esc(e.osVersion)}</span>` : '')],
    ['Processor', `${esc(e.cpu || 'Unknown')}<span class="note dim">${e.threads || '?'} threads</span>`],
    ['Graphics', gpus],
    ['Memory', ram + ramNote],
    ['Motherboard', e.board ? esc(e.board) + (e.bios ? `<span class="note dim">BIOS ${esc(e.bios)}</span>` : '') : `<span class="dim">${e.probed ? 'Unknown' : 'Checking…'}</span>`],
    ['Games', gamesFound().length ? esc(gamesFound().map(g => g.name).join(', ')) : '<span class="dim">None found yet</span>'],
    ['Discord', e.discord ? 'Installed' : '<span class="dim">Not installed</span>']
  ];
  return `<dl class="spec">${rows.map(r => `<dt>${r[0]}</dt><dd>${r[1]}</dd>`).join('')}</dl>${storageHTML()}`;
}

function healthChecks() {
  const e = S.data.env || {};
  const rp = (S.data.restorePoints || []).filter(r => r.ok).slice(-1)[0];
  const tw = (id, name, offText, onText) => {
    const s = st(id);
    if (s === 'na') return null;
    const ok = isOn(id);
    return { ok: ok ? 'ok' : 'warn', name, small: ok ? onText : offText, btn: ok ? '' : `<button class="btn sm ghost" data-act="apply1" data-id="${id}" type="button">Fix</button>` };
  };
  const list = [
    tw('power-plan', 'Power plan', 'Windows can clock your CPU down mid-game', 'AstroSpeed Performance is active'),
    tw('gamedvr', 'Background recording', 'Game DVR is recording in the background', 'Turned off'),
    tw('fn-gpupref', 'Fortnite GPU', 'Windows decides which GPU Fortnite uses', 'Always your high-performance GPU'),
    tw('nic-power', 'Network adapter power saving', 'Your network card can nap between packets', 'Turned off'),
    tw('discord-qos', 'Discord packet priority', 'Discord voice can jump ahead of game packets', 'Neutralised'),
    tw('access-shortcuts', 'Sticky Keys pop-up', 'Spamming Shift can tab you out', 'Shortcut turned off'),
    e.probed && e.ramXmp !== 'unknown' ? (e.ramXmp === 'ok'
      ? { ok: 'ok', name: 'Memory speed', small: `${e.ramConf} MT/s` }
      : { ok: 'warn', name: 'Memory speed', small: e.ramXmp === 'likely_off' ? `Running at ${e.ramConf} MT/s, which looks like XMP/EXPO is off` : `Running at ${e.ramConf} of ${e.ramSpeed} MT/s`, btn: `<button class="btn sm ghost" data-nav="cat:kernel" type="button">How to fix</button>` }) : null,
    e.probed && e.vbs >= 0 ? (e.hvci
      ? { ok: 'warn', name: 'Memory Integrity', small: 'On. It’s good security, but costs some FPS', btn: `<button class="btn sm ghost" data-nav="cat:kernel" type="button">Options</button>` }
      : { ok: 'ok', name: 'Memory Integrity', small: 'Off' }) : null,
    ...(e.gpus || []).filter(g => g.vendor && driverAgeDays(g) >= 0).slice(0, 1).map(g => {
      const days = driverAgeDays(g), months = Math.round(days / 30.4);
      return days > 180
        ? { ok: 'warn', name: 'Graphics driver', small: `${months} months old. Newer drivers often fix stutter and add game optimisations`, btn: `<button class="btn sm ghost" data-open="${DRIVER_URL[g.vendor]}" type="button">Get latest</button>` }
        : { ok: 'ok', name: 'Graphics driver', small: days < 45 ? 'Recent' : `${months} months old` };
    }),
    ...(S.drives || []).filter(d => d.freeGB / d.sizeGB < 0.1).map(d => ({ ok: 'warn', name: `Drive ${d.letter} almost full`, small: `${d.freeGB.toFixed(1)} GB free. Games can stutter when the drive is nearly full`, btn: `<button class="btn sm ghost" data-nav="cleaner" type="button">Clean up</button>` })),
    rp ? { ok: 'ok', name: 'Restore point', small: 'Last created ' + fmtDate(rp.when) }
      : { ok: 'warn', name: 'Restore point', small: 'None created by AstroSpeed yet', btn: `<button class="btn sm ghost" data-act="restorepoint" type="button">Create</button>` }
  ].filter(Boolean);
  return list.map(c => `<div class="check">${ico(c.ok === 'ok' ? 'ok' : 'warn', 'ico-' + c.ok)}<div class="t">${esc(c.name)}<small>${esc(c.small)}</small></div><div>${c.btn || ''}</div></div>`).join('');
}

// ───────────────────────── PC checkup ─────────────────────────
const WEIGHT_WORD = { 3: 'Big impact', 2: 'Medium impact', 1: 'Small impact' };
async function runScan() {
  S.scanning = true; S.checkup = null;
  if (S.page === 'checkup') rerenderView();
  const t0 = performance.now();
  const [info, startup, bg, drives] = await Promise.all([
    api('checkup').catch(() => ({})),
    api('startupList').catch(() => null),
    api('bgApps').catch(() => null),
    api('drives').catch(() => null)
  ]);
  if (startup) S.startup = startup;
  if (bg) S.bgApps = bg;
  if (drives) S.drives = drives;
  await refreshStates();
  const wait = motionOK() ? Math.max(0, 1500 - (performance.now() - t0)) : 0;
  await new Promise(r => setTimeout(r, wait));
  S.checkup = info || {}; S.scanning = false; S.scanStamp = Date.now();
  if (S.page === 'checkup') rerenderView();
}
function checkupItems() {
  const c = S.checkup || {}, e = S.data.env || {};
  const out = [];
  const add = (o) => out.push(o);
  const twCheck = (id, w, name, offText, onText) => {
    if (!byId(id) || st(id) === 'na') return;
    const ok = isOn(id);
    add({ w, ok, name, small: ok ? onText : offText, fix: ok ? '' : `<button class="btn sm ghost" data-act="apply1" data-id="${id}" type="button">Fix</button>`, tweak: ok ? '' : id });
  };
  (c.displays || []).forEach(d => {
    const low = d.maxHz > d.hz + 1;
    add({ w: 3, ok: !low, name: `${d.primary ? 'Main screen' : d.name} refresh rate`, icon: 'monitor',
      small: low ? `Running at ${d.hz} Hz, but it can do ${d.maxHz} Hz. You are seeing fewer frames than your PC draws.` : `${d.hz} Hz at ${d.width}×${d.height}, the most this screen offers`,
      fix: low ? `<button class="btn sm ghost" data-open="display" type="button">${ico('ext')}Open display settings</button>` : '' });
  });
  const goodPlan = /high|ultimate|astro|bitsum|performance/i.test(c.powerPlan || '') || isOn('power-plan');
  add({ w: 3, ok: goodPlan, name: 'Power plan', small: goodPlan ? (c.powerPlan || 'High performance') + ' is active' : `${c.powerPlan || 'Balanced'} lets Windows clock your CPU down mid-game`,
    fix: goodPlan || !byId('power-plan') || st('power-plan') === 'na' ? '' : `<button class="btn sm ghost" data-act="apply1" data-id="power-plan" type="button">Fix</button>`, tweak: goodPlan ? '' : 'power-plan' });
  if (e.probed && e.ramXmp && e.ramXmp !== 'unknown') add({ w: 3, ok: e.ramXmp === 'ok', name: 'Memory speed',
    small: e.ramXmp === 'ok' ? `${e.ramConf} MT/s, its rated speed` : e.ramXmp === 'likely_off' ? `${e.ramConf} MT/s. XMP/EXPO looks switched off in the BIOS` : `${e.ramConf} of ${e.ramSpeed} MT/s rated`,
    fix: e.ramXmp === 'ok' ? '' : `<button class="btn sm ghost" data-nav="cat:kernel" type="button">How to fix</button>` });
  if (e.ramSticks > 0) add({ w: 2, ok: e.ramSticks !== 1, name: 'Dual-channel memory', small: e.ramSticks === 1 ? 'Only one memory stick. A second matching stick can add a lot of FPS in CPU-heavy games' : `${e.ramSticks} sticks installed` });
  twCheck('gamemode', 2, 'Game Mode', 'Off. Windows won’t prioritise your game', 'On');
  twCheck('hags', 2, 'Hardware-accelerated GPU scheduling', 'Off', 'On');
  twCheck('gamedvr', 2, 'Background recording', 'Game DVR is recording in the background', 'Off');
  twCheck('nic-power', 1, 'Network adapter power saving', 'Your network card can nap between packets', 'Off');
  twCheck('fth', 1, 'Fault Tolerant Heap', 'Games that crashed before may run with extra checks', 'Off');
  (e.gpus || []).filter(g => g.vendor && driverAgeDays(g) >= 0).slice(0, 1).forEach(g => {
    const days = driverAgeDays(g), months = Math.round(days / 30.4);
    add({ w: 2, ok: days <= 180, name: 'Graphics driver', small: days <= 180 ? (days < 45 ? 'Up to date' : `${months} months old`) : `${months} months old. Newer drivers fix stutter and add game optimisations`,
      fix: days > 180 ? `<button class="btn sm ghost" data-open="${DRIVER_URL[g.vendor]}" type="button">${ico('ext')}Get latest</button>` : '' });
  });
  if (e.probed && e.vbs >= 0) add({ w: 1, ok: !e.hvci, name: 'Memory Integrity', small: e.hvci ? 'On. Good security, but it costs some FPS' : 'Off', fix: e.hvci ? `<button class="btn sm ghost" data-nav="cat:kernel" type="button">Options</button>` : '' });
  const sys = (S.drives || []).find(d => /^c/i.test(d.letter)) || (S.drives || [])[0];
  if (sys) { const pct = sys.freeGB / sys.sizeGB; add({ w: 2, ok: pct >= 0.12, name: `Free space on ${sys.letter}`, small: `${sys.freeGB.toFixed(0)} GB free of ${sys.sizeGB.toFixed(0)} GB` + (pct < 0.12 ? '. A nearly full drive causes stutter' : ''), fix: pct < 0.12 ? `<button class="btn sm ghost" data-nav="cleaner" type="button">Clean up</button>` : '' }); }
  if (S.startup) { const n = S.startup.filter(x => x.enabled).length; add({ w: 1, ok: n <= 6, name: 'Startup apps', small: `${plural(n, 'app')} start with Windows` + (n > 6 ? ', slowing boot and running in the background' : ''), fix: n > 6 ? `<button class="btn sm ghost" data-nav="startup" type="button">Review</button>` : '' }); }
  if (S.bgApps) { const mb = S.bgApps.reduce((a, b) => a + (b.memMB || 0), 0); add({ w: 1, ok: mb < 1500, name: 'Apps running in the background', small: mb ? `${fmtMB(mb)} used by ${plural(S.bgApps.length, 'app')} you could close before playing` : 'Nothing heavy running', fix: mb >= 1500 ? `<button class="btn sm ghost" data-nav="booster" type="button">Close them</button>` : '' }); }
  if (c.memLoad) add({ w: 1, ok: c.memLoad < 80, name: 'Memory in use', small: `${c.memLoad}% of your RAM is in use right now`, fix: c.memLoad >= 80 ? `<button class="btn sm ghost" data-nav="booster" type="button">Free memory</button>` : '' });
  if (c.uptimeHours) { const days = c.uptimeHours / 24; add({ w: 1, ok: days < 3, name: 'Time since restart', small: days >= 1 ? `${days.toFixed(days < 10 ? 1 : 0)} days. ` + (days >= 3 ? 'A restart clears leaked memory and stuck drivers' : 'Fine') : `${Math.max(1, Math.round(c.uptimeHours))} hours`, fix: days >= 3 ? `<button class="btn sm ghost" data-act="restart" type="button">Restart</button>` : '' }); }
  const pend = pendingFor(2).length;
  add({ w: 3, ok: pend === 0, name: 'AstroSpeed tweaks', small: pend ? `${plural(pend, 'tweak')} in the Competitive profile ${pend === 1 ? 'is' : 'are'} still off` : 'Everything in Competitive is on', fix: pend ? `<button class="btn sm ghost" data-act="optimize" type="button">Optimize</button>` : '' });
  return out;
}
function scoreOf(items) {
  const tot = items.reduce((a, b) => a + b.w, 0) || 1;
  return Math.round(items.filter(i => i.ok).reduce((a, b) => a + b.w, 0) / tot * 100);
}
function gradeOf(n) { return n >= 90 ? 'Tournament ready' : n >= 75 ? 'In good shape' : n >= 55 ? 'Room to improve' : 'Holding you back'; }
function viewCheckup() {
  const R = 84, C = 2 * Math.PI * R;
  if (!S.checkup) {
    return `<div class="panel spot scan-panel"><div class="scan-ring"><svg viewBox="0 0 200 200"><circle class="trk" cx="100" cy="100" r="${R}"/><circle class="sweep" cx="100" cy="100" r="${R}" style="stroke-dasharray:${C * .22} ${C}"/></svg><div class="scan-mid">${ico('pulse')}</div></div>
      <div class="scan-copy"><h2>Scanning your PC</h2><p class="muted">Screens, power, memory, drivers, storage, startup apps and what's running in the background.</p><div class="scan-steps">${['Displays', 'Power plan', 'Memory', 'Drivers', 'Storage', 'Startup', 'Background apps'].map((x, i) => `<span style="--i:${i}">${x}</span>`).join('')}</div></div></div>`;
  }
  const items = checkupItems();
  const score = scoreOf(items);
  const bad = items.filter(i => !i.ok), fixable = bad.filter(i => i.tweak);
  items.sort((a, b) => (a.ok - b.ok) || (b.w - a.w));
  const tone = score >= 90 ? 'ok' : score >= 55 ? 'mid' : 'low';
  return `
  <div class="panel spot score-panel tone-${tone}">
    <div class="score-ring"><svg viewBox="0 0 200 200"><defs><linearGradient id="sg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="var(--accent)"/><stop offset="1" stop-color="#FFB37A"/></linearGradient></defs>
      <circle class="trk" cx="100" cy="100" r="${R}"/><circle class="val" cx="100" cy="100" r="${R}" style="stroke-dasharray:${C};stroke-dashoffset:${C * (1 - score / 100)};--from:${C}"/></svg>
      <div class="score-mid"><b data-score="${score}">${score}</b><span>out of 100</span></div></div>
    <div class="score-copy">
      <div class="eyebrow">Your score</div>
      <h2>${esc(gradeOf(score))}</h2>
      <p class="muted">${bad.length ? `${plural(bad.length, 'thing')} could be costing you frames.${fixable.length ? ` ${fixable.length} can be fixed in one click.` : ''}` : 'Nothing is holding this PC back. Run it again after big Windows or driver updates.'}</p>
      <div class="actions">${fixable.length ? `<button class="btn primary" data-act="fixAll" type="button">${ico('check')}Fix ${plural(fixable.length, 'issue')}</button>` : ''}<button class="btn ghost" data-act="rescan" type="button">${ico('reboot')}Scan again</button></div>
    </div>
    <div class="score-legend">${[['ok', items.filter(i => i.ok).length, 'Passed'], ['warn', bad.length, 'To fix']].map(([k, n, l]) => `<div><b class="ico-${k}" data-count="${n}">${n}</b><span>${l}</span></div>`).join('')}</div>
  </div>
  <div class="cu-list">
    ${items.map((c, i) => `<div class="cu-row ${c.ok ? 'ok' : 'warn'}" style="--i:${i}">
      <span class="cu-ico">${ico(c.ok ? 'ok' : 'warn', c.ok ? 'ico-ok' : 'ico-warn')}</span>
      <div class="t"><b>${esc(c.name)}</b><small>${esc(c.small)}</small></div>
      <span class="cu-w w${c.w}">${c.ok ? '' : WEIGHT_WORD[c.w]}</span>
      <div class="cu-fix">${c.fix || ''}</div>
    </div>`).join('')}
  </div>`;
}
function animateCheckup() {
  const fresh = S.scanStamp; S.scanStamp = 0;
  const v = $('#view');
  if (!fresh || !motionOK()) return;
  v.classList.add('cu-anim');
  const b = $('[data-score]', v), ring = $('.score-ring .val', v);
  if (b) { b.textContent = '0'; countUp(b, +b.dataset.score, 1700, 250); }
  if (ring) { const to = ring.style.strokeDashoffset; ring.style.transition = 'none'; ring.style.strokeDashoffset = ring.style.getPropertyValue('--from'); void ring.getBoundingClientRect(); ring.style.transition = ''; requestAnimationFrame(() => { ring.style.strokeDashoffset = to; }); }
  $$('.score-legend [data-count]', v).forEach((el, i) => countUp(el, +el.dataset.count, 1200, 600 + i * 150));
  setTimeout(() => v.classList.remove('cu-anim'), 3000);
}

// ───────────────────────── live ping graph ─────────────────────────
const PING_REGIONS = [['oce', 'Oceania'], ['asia', 'Asia'], ['me', 'Middle East'], ['eu', 'Europe'], ['nae', 'NA East'], ['nac', 'NA Central'], ['naw', 'NA West'], ['br', 'Brazil']];
function livePingPanel() {
  const L = S.livePing;
  return `<div class="panel spot live-ping">
    <div class="panel-h"><h2>Live ping</h2>
      <div class="lp-ctl"><select id="lpRegion" aria-label="Region">${PING_REGIONS.map(([k, n]) => `<option value="${k}"${L.region === k ? ' selected' : ''}>${n}</option>`).join('')}</select>
      <button class="btn sm ${L.running ? 'ghost' : 'primary'}" data-act="livePing" type="button">${L.running ? ico('pause') + 'Stop' : ico('play') + 'Start'}</button></div></div>
    <div class="panel-b">
      <div class="lp-stats" id="lpStats"></div>
      <div class="lp-chart" id="lpChart"></div>
      <p class="muted small" style="margin:10px 0 0">One ping every second to the Fortnite servers for the region you pick. Spikes and gaps show Wi-Fi drops, someone downloading at home, or a busy connection.</p>
    </div></div>`;
}
function drawLivePing(full) {
  const box = $('#lpChart'), stats = $('#lpStats');
  if (!box) return;
  const L = S.livePing, xs = L.samples, N = 60;
  const good = xs.filter(x => x != null);
  const avg = good.length ? good.reduce((a, b) => a + b, 0) / good.length : 0;
  let jit = 0; for (let i = 1; i < good.length; i++) jit += Math.abs(good[i] - good[i - 1]); jit = good.length > 1 ? jit / (good.length - 1) : 0;
  const loss = xs.length ? Math.round((xs.length - good.length) / xs.length * 100) : 0;
  const last = xs.length ? xs[xs.length - 1] : null;
  stats.innerHTML = [['Now', last == null ? '–' : Math.round(last) + ' ms', true], ['Average', good.length ? Math.round(avg) + ' ms' : '–'], ['Best', good.length ? Math.round(Math.min(...good)) + ' ms' : '–'], ['Jitter', good.length > 1 ? jit.toFixed(1) + ' ms' : '–'], ['Loss', xs.length ? loss + '%' : '–']]
    .map(([l, v, big]) => `<div${big ? ' class="big"' : ''}><span>${l}</span><b>${v}</b></div>`).join('');
  const W = 1000, H = 180, pad = 10;
  const max = Math.max(40, ...good) * 1.25;
  const X = i => (i + (N - xs.length)) / (N - 1) * W;
  const Y = v => H - pad - v / max * (H - pad * 2);
  let d = '', area = '', started = false, firstX = 0;
  xs.forEach((v, i) => { if (v == null) { started = false; return; } const p = `${X(i).toFixed(1)},${Y(v).toFixed(1)}`; if (!started) { d += 'M' + p; if (!area) { firstX = X(i); area = 'M' + p; } else area += 'L' + p; started = true; } else { d += 'L' + p; area += 'L' + p; } });
  if (area) area += `L${X(xs.length - 1).toFixed(1)},${H}L${firstX.toFixed(1)},${H}Z`;
  const grid = [0.25, 0.5, 0.75].map(f => `<line x1="0" x2="${W}" y1="${(H * f).toFixed(0)}" y2="${(H * f).toFixed(0)}"/>`).join('');
  const lost = xs.map((v, i) => v == null ? `<rect x="${(X(i) - 3).toFixed(1)}" y="0" width="6" height="${H}"/>` : '').join('');
  const tip = last != null ? `<i class="lp-dot" style="left:${(X(xs.length - 1) / W * 100).toFixed(2)}%;top:${(Y(last) / H * 100).toFixed(2)}%"></i>` : '';
  box.innerHTML = `<div class="lp-plot"><svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none"><defs><linearGradient id="lpg" x1="0" x2="0" y1="0" y2="1"><stop offset="0" stop-color="var(--accent)" stop-opacity=".28"/><stop offset="1" stop-color="var(--accent)" stop-opacity="0"/></linearGradient></defs>
    <g class="lp-grid">${grid}</g><g class="lp-lost">${lost}</g>${area ? `<path class="lp-area" d="${area}"/>` : ''}${d ? `<path class="lp-line" d="${d}" vector-effect="non-scaling-stroke"/>` : ''}</svg>${tip}</div>
    <div class="lp-y"><span>${Math.round(max)} ms</span><span>${Math.round(max / 2)} ms</span><span>0</span></div>
    ${xs.length ? '' : `<div class="lp-empty">Press Start to watch your ping live</div>`}`;
}
async function livePingLoop() {
  const L = S.livePing;
  while (L.running) {
    const t0 = performance.now();
    let v = null;
    try { const r = await api('pingOnce', { id: L.region }); v = r && !r.error && r.loss < 100 ? r.avg : null; } catch (e) { v = null; }
    if (!L.running) break;
    L.samples.push(v); if (L.samples.length > 60) L.samples.shift();
    if (S.page === 'network') drawLivePing(); else { L.running = false; break; }
    await new Promise(r => setTimeout(r, Math.max(0, 1000 - (performance.now() - t0))));
  }
}

// ───────────────────────── settings ─────────────────────────
function setRow(key, title, desc, on, extra = '') {
  return `<div class="set-row"><div class="set-t"><b>${esc(title)}</b><span>${desc}</span>${extra}</div><button class="switch" role="switch" aria-checked="${!!on}" data-act="setting" data-key="${key}" type="button" aria-label="${esc(title)}"></button></div>`;
}
function viewSettings() {
  const d = S.data, u = S.update;
  const theme = document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
  const motion = !document.documentElement.classList.contains('motion-off');
  let upd;
  if (!d.updateConfigured) upd = `<p class="muted small" style="margin:0">Automatic updates aren't available in this copy.</p>`;
  else if (S.checking) upd = `<p class="muted" style="margin:0;display:flex;gap:10px;align-items:center"><span class="spin"></span>Checking…</p>`;
  else if (u && u.error) upd = `<p class="muted" style="margin:0">${esc(u.error)}</p>`;
  else if (u && u.available) upd = `<p style="margin:0"><b>Version ${esc(u.latest)} is available.</b> <span class="muted">You have ${esc(u.current)}.</span></p><div class="actions"><button class="btn sm primary" data-act="showUpdate" type="button">${ico('arrow')}See what's new</button></div>`;
  else if (u) upd = `<p class="muted" style="margin:0">${ico('ok', 'ico-ok')} You're on the latest version (${esc(d.version)}).</p>`;
  else upd = `<p class="muted" style="margin:0">Not checked yet.</p>`;
  return `
  <div class="grid settings-grid">
    ${licensePanel(d)}
    <div class="panel spot"><div class="panel-h"><h2>Appearance</h2></div><div class="panel-b">
      <div class="set-row"><div class="set-t"><b>Theme</b><span>Paper is light, Charcoal is dark.</span></div>
        <div class="seg"><button class="${theme === 'light' ? 'on' : ''}" data-act="theme" data-v="light" type="button">Paper</button><button class="${theme === 'dark' ? 'on' : ''}" data-act="theme" data-v="dark" type="button">Charcoal</button></div></div>
      ${setRow('motion', 'Animations', 'Page transitions, the dashboard flight and floating cards.', motion)}
    </div></div>
    <div class="panel spot"><div class="panel-h"><h2>Updates</h2>${d.updateConfigured ? `<button class="btn sm quiet" data-act="checkUpdate" type="button"${S.checking ? ' disabled' : ''}>${ico('reboot')}Check now</button>` : ''}</div><div class="panel-b">
      ${upd}
      ${d.updateConfigured ? setRow('autoUpdate', 'Check for updates automatically', 'Lets you know when a new version of AstroSpeed is ready.', d.autoUpdate) : ''}
    </div></div>
    ${packPanel(d)}
    <div class="panel spot"><div class="panel-h"><h2>Boosting</h2></div><div class="panel-b">
      ${setRow('hotkey', 'Boost hotkey: Ctrl + Alt + B', 'Press it any time, even in a game, to clear memory, tighten the timer, flush DNS and raise the running game to high priority. You hear a short sound when it’s done.', d.hotkey)}
      ${setRow('autoboost', 'Boost automatically when a game starts', 'While AstroSpeed is open, any supported game is boosted the moment it launches.', d.autoBoost)}
    </div></div>
    <div class="panel spot"><div class="panel-h"><h2>Startup</h2></div><div class="panel-b">
      ${setRow('startup', 'Start with Windows', 'Opens AstroSpeed minimised when you sign in, so the hotkey and auto-boost are always ready. No admin prompt each time.', S.startupOn)}
      <div class="actions"><button class="btn sm ghost" data-open="logs" type="button">${ico('log')}Open logs and backups</button></div>
    </div></div>
  </div>`;
}

function licensePanel(d) {
  const l = d.license || { required: false, activated: true };
  if (!l.required) return '';
  const body = l.activated
    ? `<p style="margin:0;display:flex;align-items:center;gap:8px">${ico('ok','ico-ok')}<b>Full version unlocked</b>${l.offline ? ' <span class="muted">(offline — using your saved key)</span>' : ''}</p><p class="muted small" style="margin:8px 0 0">Key ${esc(l.key||'')} &middot; locked to this PC.</p>`
    : `<p style="margin:0"><b>You're on the free version.</b></p><p class="muted small" style="margin:8px 0 14px">A key unlocks the full Optimize, all per-game tweaks, network/BIOS tweaks, auto-boost and the hotkey.</p><div class="actions" style="margin:0"><button class="btn sm primary" data-act="upgrade" type="button">${ico('lock')}Enter a key</button><button class="btn sm ghost" data-act="buyKey" type="button">${ico('ext')}Get a key</button></div>`;
  return `<div class="panel spot"><div class="panel-h"><h2>Your key</h2></div><div class="panel-b">${body}</div></div>`;
}
function packPanel(d) {
  const p = d.pack || {};
  let body;
  if (!d.updateRepo) body = `<p class="muted small" style="margin:0">Live tweaks aren't available in this copy.</p>`;
  else {
    const when = p.checked ? new Date(p.checked).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';
    const stat = S.packChecking ? `<span class="spin"></span>Checking…`
      : p.error ? esc(p.error) + (p.found ? '. Using the last copy downloaded.' : '')
      : !p.found ? (p.checked ? 'No live tweaks right now. New ones show up here by themselves.' : 'Not checked yet.')
      : `${ico('ok', 'ico-ok')} Up to date${when ? ' · checked ' + esc(when) : ''}`;
    body = `<p class="muted" style="margin:0;display:flex;gap:10px;align-items:center;flex-wrap:wrap">${stat}</p>
      ${p.found ? `<div class="pack-stats"><div><b>${p.tweaks || 0}</b><span>tweaks</span></div><div><b>${p.games || 0}</b><span>games</span></div><div><b>${p.sections || 0}</b><span>sections</span></div><div><b>${Object.keys(p.images || {}).length}</b><span>pictures</span></div></div>` : ''}
      ${(p.problems || []).length ? `<details class="pack-probs"><summary>${plural(p.problems.length, 'line')} skipped</summary>${p.problems.slice(0, 40).map(x => `<div>${esc(x)}</div>`).join('')}</details>` : ''}
      <p class="muted small" style="margin:0">New tweaks arrive by themselves, no app update needed. Nothing is applied until you switch it on or run Optimize.</p>`;
  }
  return `<div class="panel spot"><div class="panel-h"><h2>Live tweaks</h2>${d.updateRepo ? `<button class="btn sm quiet" data-act="refreshPack" type="button"${S.packChecking ? ' disabled' : ''}>${ico('reboot')}Check now</button>` : ''}</div><div class="panel-b">${body}</div></div>`;
}
async function reloadCatalog(info) {
  try {
    const nd = await api('init');
    S.data = Object.assign(S.data, { categories: nd.categories, catalog: nd.catalog, games: nd.games, tweakCount: nd.tweakCount, actionCount: nd.actionCount, pack: nd.pack || info });
    const r = await api('states', { bcd: false });
    S.states = r.states || S.states;
    try { S.data.env.games = await api('games'); } catch (e) {}
    if (S.page.startsWith('cat:') && !S.data.categories.some(c => 'cat:' + c.id === S.page)) S.page = 'dashboard';
    renderNav(); renderBanners(); rerenderView();
  } catch (e) { /* keep what is on screen */ }
}

// ───────────────────────── licence (free vs key) ─────────────────────────
function licenseChip() {
  const l = LIC();
  if (!l.required) return '';
  if (l.activated) return `<div class="lic-chip on">${ico('ok','ico-ok')}<span>Full version${l.offline ? ' (offline)' : ''}</span></div>`;
  return `<button class="lic-chip buy" data-act="upgrade" type="button">${ico('lock')}<span>Free version &middot; Unlock</span></button>`;
}
function showUpgrade() {
  const proList = T().filter(t => t.pro);
  openModal(`
    <div class="dialog-h"><div><div class="eyebrow">AstroSpeed key</div><h2>Unlock the full version</h2><p>The free version gives you the dashboard, PC Checkup, Game Booster, cleaner and the safe recommended tweaks. A key unlocks everything else.</p></div><button class="btn quiet x" data-close type="button" aria-label="Close">${ico('x')}</button></div>
    <div class="dialog-b">
      <div class="up-grid">
        <div class="up-col"><h4>Free</h4><ul>${['Dashboard & PC Checkup','Game Booster & cleaner','Safe recommended tweaks','Ping test'].map(x=>`<li>${ico('check','ico-ok')}${x}</li>`).join('')}</ul></div>
        <div class="up-col pro"><h4>${ico('lock')}With a key</h4><ul>${['Full one-click Optimize (Competitive)','All per-game tweaks','Network, GPU & BIOS tweaks','Auto-boost & the boost hotkey','Live tweaks sent to the app'].map(x=>`<li>${ico('check','ico-ok')}${x}</li>`).join('')}</ul></div>
      </div>
      <label class="key-in"><span>Already bought a key? Enter it:</span>
        <input id="keyInput" type="text" autocomplete="off" spellcheck="false" placeholder="ASTRO-XXXXX-XXXXX-XXXXX" value="${esc((LIC().key||''))}">
      </label>
      <div id="keyErr" class="key-err"></div>
    </div>
    <div class="dialog-f"><button class="btn ghost" data-act="buyKey" type="button">${ico('ext')}Get a key</button><span class="spacer"></span><button class="btn secondary" data-close type="button">Not now</button><button class="btn primary" data-act="activateKey" data-autofocus type="button">${ico('check')}Activate</button></div>`);
  const inp = $('#keyInput'); if (inp) { inp.focus(); inp.addEventListener('keydown', e => { if (e.key === 'Enter') doActivate(); }); }
}
async function doActivate() {
  const inp = $('#keyInput'), err = $('#keyErr');
  if (!inp) return;
  const key = inp.value.trim();
  if (key.length < 5) { err.textContent = 'Enter your key first.'; return; }
  err.textContent = ''; const btn = $('[data-act="activateKey"]'); if (btn) { btn.disabled = true; btn.innerHTML = '<span class="spin"></span>Checking…'; }
  try {
    const r = await api('activate', { key });
    if (r && r.activated) {
      closeModal();
      S.data.license = r;
      toast('Unlocked! Thanks for supporting AstroSpeed.', 'ok', 5000);
      const nd = await api('init').catch(() => null);
      if (nd) { S.data = Object.assign(S.data, { catalog: nd.catalog, license: nd.license }); }
      renderNav(); rerenderView();
    } else { err.textContent = (r && r.error) || 'That key didn\'t work.'; }
  } catch (e) { err.textContent = e.message || 'Couldn\'t reach the server.'; }
  if (btn) { btn.disabled = false; btn.innerHTML = `${ico('check')}Activate`; }
}

// ───────────────────────── update prompt ─────────────────────────
function notesHTML(md) {
  const lines = String(md || '').replace(/\r/g, '').split('\n').map(l => l.trim()).filter(Boolean).slice(0, 30);
  if (!lines.length) return '<p class="muted">Bug fixes and improvements.</p>';
  let out = '', list = false;
  for (const l of lines) {
    const bullet = /^[-*•]\s+/.test(l);
    const text = esc(l.replace(/^[-*•]\s+/, '').replace(/^#+\s*/, '')).replace(/\*\*(.+?)\*\*/g, '<b>$1</b>');
    if (bullet && !list) { out += '<ul class="notes">'; list = true; }
    if (!bullet && list) { out += '</ul>'; list = false; }
    out += bullet ? `<li>${text}</li>` : (/^#/.test(l) ? `<h4>${text}</h4>` : `<p>${text}</p>`);
  }
  return out + (list ? '</ul>' : '');
}
function showUpdate() {
  const u = S.update;
  if (!u || !u.available) return;
  openModal(`
    <div class="dialog-h"><div class="upd-mark">${ico('arrow')}</div><div><h2>AstroSpeed ${esc(u.latest)} is ready</h2><p>You have version ${esc(u.current)}. Your tweaks, backups and settings carry over.</p></div><button class="btn quiet x" data-close type="button" aria-label="Close">${ico('x')}</button></div>
    <div class="dialog-b"><div class="upd-notes">${u.title && u.title !== u.latest && u.title !== 'v' + u.latest ? `<h3>${esc(u.title)}</h3>` : ''}${notesHTML(u.notes)}</div>
      <div class="upd-prog" id="updProg" hidden><div class="upd-bar"><i id="updBarFill"></i></div><div class="upd-prog-txt" id="updProgTxt">Starting…</div></div></div>
    <div class="dialog-f" id="updFoot"><button class="btn quiet" data-act="skipUpdate" type="button">Skip this version</button><span class="spacer"></span><button class="btn ghost" data-close type="button">Later</button><button class="btn primary" data-act="installUpdate" data-autofocus type="button">${ico('arrow', 'down')}Update now</button></div>`, true);
}
let updating = false;
on('updateProgress', p => {
  const box = $('#updProg'), fill = $('#updBarFill'), txt = $('#updProgTxt');
  if (!box) return;
  box.hidden = false;
  const pct = Math.max(0, Math.min(100, p.pct || 0));
  if (fill) fill.style.width = pct + '%';
  if (txt) txt.textContent = p.done ? 'Finishing…' : (p.totalMB ? `Downloading… ${pct}% (${(p.receivedMB||0).toFixed(1)} of ${(p.totalMB||0).toFixed(1)} MB)` : `Downloading… ${pct}%`);
});
async function runUpdate() {
  if (updating) return; updating = true;
  const foot = $('#updFoot'), prog = $('#updProg');
  const m = $('#modal'); if (m) m.dataset.locked = '1';
  if (foot) foot.innerHTML = `<span class="muted small">Keep AstroSpeed open while it updates…</span>`;
  if (prog) prog.hidden = false;
  try {
    await api('downloadUpdate');
    const txt = $('#updProgTxt'); if (txt) txt.textContent = 'Installing and restarting…';
    await api('applyUpdate');          // app relaunches itself; this window closes
  } catch (e) {
    updating = false;
    if (m) delete m.dataset.locked;
    if (prog) prog.hidden = true;
    if (foot) foot.innerHTML = `<button class="btn quiet" data-act="skipUpdate" type="button">Skip this version</button><span class="spacer"></span><button class="btn ghost" data-close type="button">Later</button><button class="btn primary" data-act="installUpdate" type="button">Try again</button>`;
    toast(e.message || 'Update failed.', 'err', 8000);
  }
}
async function autoCheckUpdate() {
  if (!S.data.updateConfigured || !S.data.autoUpdate) return;
  try {
    S.update = await api('checkUpdate');
    if (!S.update.available) return;
    renderNav();
    let skipped = '';
    try { skipped = localStorage.getItem('sg-skip-version') || ''; } catch (e) { /* ignore */ }
    if (skipped !== S.update.latest) setTimeout(() => { if ($('#modal').hidden) showUpdate(); }, 1800);
  } catch (e) { /* offline: try again next launch */ }
}

// ───────────────────────── storage & driver checks ─────────────────────────
function storageHTML() {
  const ds = S.drives;
  if (!ds || !ds.length) return '';
  return `<div class="storage"><div class="st-h">Storage</div>${ds.map(d => {
    const used = 1 - d.freeGB / d.sizeGB, low = d.freeGB / d.sizeGB < 0.1;
    return `<div class="drive${low ? ' low' : ''}"><div class="dr-top"><b>${esc(d.letter)}</b><span>${esc(d.label || 'Local disk')}</span><em>${d.freeGB >= 100 ? Math.round(d.freeGB) : d.freeGB.toFixed(1)} GB free of ${Math.round(d.sizeGB)} GB</em></div><div class="meterbar"><i style="width:${Math.round(used * 100)}%"></i></div></div>`;
  }).join('')}</div>`;
}
function driverAgeDays(g) {
  const m = /^(\d{1,2})-(\d{1,2})-(\d{4})$/.exec((g && g.driverDate) || '');
  if (!m) return -1;
  return Math.floor((Date.now() - new Date(+m[3], +m[1] - 1, +m[2]).getTime()) / 864e5);
}
const DRIVER_URL = { nvidia: 'https://www.nvidia.com/en-us/drivers/', amd: 'https://www.amd.com/en/support/download/drivers.html', intel: 'https://www.intel.com/content/www/us/en/support/detect.html' };

// ───────────────────────── games ─────────────────────────
const gameDef = id => (S.data.games || []).find(g => g.id === id);
const gameCatOf = id => S.data.categories.find(c => c.game === id);
const isGameCat = page => { const c = page && page.startsWith('cat:') && S.data.categories.find(x => x.id === page.slice(4)); return !!(c && c.game); };
const gamesFound = () => (S.data.games || []).filter(g => ((S.data.env || {}).games || {})[g.id]);
function initials(name) {
  const w = String(name).split(/[\s\-]+/).filter(Boolean);
  return (w.length > 1 ? w[0][0] + w[1][0] : w[0].slice(0, 2)).toUpperCase();
}
// how each supplied picture is framed: banner (wide card), square (small tiles)
const ART_FIT = {
  fortnite: { banner: '50% 48%', square: '50% 55%' },
  apex: { banner: '50% 4%', square: '50% 0%' },
  rust: { banner: '50% 86%', square: '50% 50%' },
  cs2: { banner: '50% 0%', square: '50% 50%' },
  roblox: { contain: true, bg: '#335FFF', pad: '16%' },
  valorant: { contain: true, bg: '#FFFFFF', pad: '18%' },
  lol: { banner: '50% 75%', square: '92% 50%' }
};
// games without a bundled picture use their official Steam store image, loaded when the page shows
const STEAM_APP = { sot: 1172620, overwatch: 2357570, rivals: 2767030, rocketleague: 252950, gta: 3240220 };
const steamArt = id => STEAM_APP[id] ? `https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/${STEAM_APP[id]}/header.jpg` : '';
const steamArtAlt = id => STEAM_APP[id] ? `https://cdn.cloudflare.steamstatic.com/steam/apps/${STEAM_APP[id]}/header.jpg` : '';
window.artFail = img => { if (img.dataset.alt && img.src !== img.dataset.alt) { img.src = img.dataset.alt; img.dataset.alt = ''; } else img.remove(); };
function steamImg(id, pos) {
  return `<img class="art steam" src="${steamArt(id)}" data-alt="${steamArtAlt(id)}" onerror="artFail(this)" alt="" draggable="false" style="object-fit:cover;object-position:${pos}">`;
}
const packImg = id => { const p = (S.data && S.data.pack) || {}; if ((p.images || {})[id]) return p.images[id]; const g = ((S.data && S.data.games) || []).find(x => x.id === id); return g && g.image ? g.image : ''; };
const artOf = id => packImg(id) || (window.__ART || {})[id];
function artImg(id, kind) {
  const src = artOf(id), f = packImg(id) ? {} : (ART_FIT[id] || {});
  if (!src) return '';
  if (packImg(id)) return `<img class="art" src="${esc(src)}" onerror="artFail(this)" alt="" draggable="false" style="object-fit:cover;object-position:50% 50%">`;
  let style = `object-fit:cover;object-position:${f[kind] || '50% 50%'}`;
  if (f.contain) style = kind === 'banner' ? 'object-fit:contain;inset:22% 0;height:56%' : `object-fit:contain;padding:${f.pad || 0}`;
  return `<img class="art" src="${src}" alt="" draggable="false" style="${style}">`;
}
const artBg = id => packImg(id) ? '' : ((ART_FIT[id] || {}).bg || '');
function gameBanner(g) {
  const art = artImg(g.id, 'banner');
  if (art) return `<span class="gbanner"${artBg(g.id) ? ` style="background:${artBg(g.id)}"` : ''}>${art}</span>`;
  const icon = (S.icons || {})[g.id];
  return `<span class="gbanner plain" style="--gc:${g.color}">${icon ? `<img class="ico" src="${icon}" alt="" draggable="false">` : `<b>${esc(initials(g.name))}</b>`}${STEAM_APP[g.id] ? steamImg(g.id, '50% 50%') : ''}</span>`;
}
function gameTile(g, size) {
  if (artOf(g.id)) return `<span class="gicon art" style="--s:${size}px${artBg(g.id) ? ';background:' + artBg(g.id) : ''}">${artImg(g.id, 'square')}</span>`;
  const icon = (S.icons || {})[g.id];
  const steam = STEAM_APP[g.id] ? steamImg(g.id, '50% 50%') : '';
  return icon
    ? `<span class="gicon" style="--s:${size}px"><img src="${icon}" alt="" draggable="false">${steam}</span>`
    : `<span class="gicon mono" style="--s:${size}px;--gc:${g.color}">${esc(initials(g.name))}${steam}</span>`;
}

function viewGames() {
  const games = (S.data.games || []).slice();
  const found = gamesFound();
  const env = S.data.env || {};
  games.sort((a, b) => (!!(env.games || {})[b.id]) - (!!(env.games || {})[a.id]));
  const pend = gamePending().length;
  return `
  <div class="toolbar">
    <span class="sum"><b>${found.length}</b> of ${games.length} supported games found on this PC</span>
    <span class="spacer"></span>
    <button class="btn sm quiet" data-act="rescanGames" type="button">${ico('reboot')}Look for games again</button>
    <button class="btn sm primary" data-act="applyGames" type="button"${pend ? '' : ' disabled'}>${ico('check')}Turn on ${pend ? plural(pend, 'game tweak') : 'game tweaks'}</button>
  </div>
  <div class="games-grid">
    ${games.map(g => {
      const cat = gameCatOf(g.id);
      const list = cat ? catTweaks(cat.id) : [];
      const has = !!(env.games || {})[g.id];
      const a = appliedCount(list), n = list.length;
      return `<button class="gcard panel spot${has ? '' : ' missing'}" data-nav="cat:${cat ? cat.id : ''}" type="button">
        ${gameBanner(g)}
        <span class="gname">${esc(g.name)}</span>
        <span class="gstudio">${esc(g.studio)}</span>
        <span class="gfoot">${has ? `<span class="pill ok">${ico('check')}Installed</span><span class="gcount"><b>${a}</b>/${n} on</span>` : '<span class="pill">Not found</span>'}</span>
        <span class="gbar"><i style="width:${n ? Math.round(a / n * 100) : 0}%"></i></span>
      </button>`;
    }).join('')}
  </div>
  <p class="dim small" style="margin-top:18px">Every game gets the same five process tweaks: GPU choice, CPU and disk priority, fullscreen behaviour and network tagging. None of them open the game's files, its settings or its anti-cheat. Some pictures load from the Steam store when you're online.</p>`;
}
function gamePending() {
  const p = S.data.profile || 2;
  return T().filter(t => isGameCat('cat:' + t.cat) && t.profile > 0 && t.profile <= p && avail(t.id) && !isOn(t.id));
}

function gameIntro(id) {
  const g = gameDef(id) || { id, name: id, studio: '', color: '#888' };
  const exe = ((S.data.env || {}).games || {})[id] || (id === 'fortnite' ? (S.data.env || {}).fortniteExe : '');
  return `<div class="intro">
    <div class="panel spot game-head">${gameBanner(g)}<div class="panel-b">
      <div class="gh-top">${gameTile(g, 84)}<div><div class="gh-name">${esc(g.name)}</div><div class="gstudio">${esc(g.studio)}</div></div></div>
      <dl class="kv" style="margin-top:18px"><dt>Game file</dt><dd>${exe ? `<span class="path" title="${esc(exe)}">${esc(exe)}</span>` : `<span class="dim">Not found on this PC. Its tweaks unlock when it's installed.</span>`}</dd></dl>
      <div class="actions">
        ${exe ? `<button class="btn sm primary" data-open="game:${id}" type="button">${ico('glider')}Launch ${esc(g.name)}</button><button class="btn sm ghost" data-open="gamedir:${id}" type="button">${ico('ext')}Open folder</button>` : ''}
        <button class="btn sm quiet" data-act="rescanGames" type="button">${ico('reboot')}Look again</button>
        <button class="btn sm quiet" data-nav="games" type="button">All games</button>
      </div>
    </div></div>
    <div class="panel spot"><div class="panel-h"><h2>Your game settings stay yours</h2></div><div class="panel-b">
      <p class="muted" style="margin:0">These tweaks change how Windows treats ${esc(g.name)}: its priority, which GPU it uses and how its packets are tagged. AstroSpeed never opens the game's settings, so your sensitivity, keybinds, graphics settings and crosshair are exactly as you left them.</p>
      ${g.note ? `<p class="note-line">${ico('info')}<span>${esc(g.note)}</span></p>` : ''}
    </div></div></div>`;
}

// ───────────────────────── live monitor ─────────────────────────
let liveTimer = 0;
S.hist = { cpu: [], ram: [] };
function liveHTML() {
  return `<div class="panel spot live"><div class="panel-h"><h2>Live</h2><span class="live-dot"><i></i>Updating every second</span></div><div class="panel-b live-grid">
    <div class="lv"><span class="lv-l">CPU</span><span class="lv-v" id="lvCpu">–</span><svg class="spark" id="spCpu" viewBox="0 0 120 34" preserveAspectRatio="none"></svg></div>
    <div class="lv"><span class="lv-l">Memory</span><span class="lv-v" id="lvRam">–</span><svg class="spark" id="spRam" viewBox="0 0 120 34" preserveAspectRatio="none"></svg></div>
    <div class="lv"><span class="lv-l">Timer</span><span class="lv-v" id="lvTimer">–</span><span class="lv-s" id="lvTimerS">ms resolution</span></div>
    <div class="lv"><span class="lv-l">Now playing</span><span class="lv-v lv-game" id="lvGame">–</span><span class="lv-s" id="lvProcs"></span></div>
  </div></div>`;
}
function sparkPath(arr) {
  if (arr.length < 2) return '';
  const n = 40, pts = arr.slice(-n), w = 120, h = 34;
  const step = w / (n - 1), off = (n - pts.length) * step;
  const line = pts.map((v, i) => `${(off + i * step).toFixed(1)},${(h - 2 - (Math.max(0, Math.min(100, v)) / 100) * (h - 4)).toFixed(1)}`).join(' ');
  return `<polyline class="area" points="${off.toFixed(1)},${h} ${line} ${w},${h}"/><polyline class="ln" points="${line}"/>`;
}
async function liveTick() {
  if (S.page !== 'dashboard' || !$('#lvCpu')) { stopLive(); return; }
  try {
    const l = await api('live');
    if (l.cpu >= 0) { if (!S.hist.cpu.length) S.hist.cpu = Array(40).fill(l.cpu); S.hist.cpu.push(l.cpu); }
    if (!S.hist.ram.length) S.hist.ram = Array(40).fill(l.ramPct);
    S.hist.ram.push(l.ramPct);
    S.hist.cpu = S.hist.cpu.slice(-60); S.hist.ram = S.hist.ram.slice(-60);
    const set = (id, v) => { const el = $('#' + id); if (el && el.innerHTML !== v) el.innerHTML = v; };
    set('lvCpu', l.cpu >= 0 ? `${Math.round(l.cpu)}<small>%</small>` : '…');
    set('lvRam', `${Math.round(l.ramPct)}<small>%</small>`);
    set('lvTimer', l.timer ? l.timer.toFixed(2) : '–');
    set('lvGame', l.game ? esc(l.game) : '<span class="dim">Nothing</span>');
    set('lvProcs', `${l.procs} processes running · ${l.ramUsedGB.toFixed(1)} of ${Math.round(l.ramGB)} GB used`);
    const a = $('#spCpu'), b = $('#spRam');
    if (a) a.innerHTML = sparkPath(S.hist.cpu);
    if (b) b.innerHTML = sparkPath(S.hist.ram);
  } catch (e) { /* the monitor is optional */ }
}
function startLive() { stopLive(); liveTick(); liveTimer = setInterval(liveTick, 1000); }
function stopLive() { clearInterval(liveTimer); liveTimer = 0; }

function yourGamesHTML() {
  const found = gamesFound();
  const body = found.length
    ? `<div class="ygames">${found.slice(0, 8).map(g => { const c = gameCatOf(g.id); const l = c ? catTweaks(c.id) : []; return `<button class="yg" data-nav="cat:${c ? c.id : ''}" type="button">${gameTile(g, 52)}<span>${esc(g.name)}</span><small>${appliedCount(l)}/${l.length} on</small></button>`; }).join('')}</div>`
    : `<p class="muted" style="margin:6px 0 0">No supported games found yet. AstroSpeed looks in Steam, Epic, Riot, Battle.net, EA and Roblox folders.</p>`;
  return `<div class="panel spot"><div class="panel-h"><h2>Your games</h2><button class="btn sm quiet" data-nav="games" type="button">All games${ico('arrow', 'arr')}</button></div><div class="panel-b">${body}</div></div>`;
}

// ───────────────────────── tweak rows ─────────────────────────
const impactWord = ['', 'Low', 'Medium', 'High'];
const profileWord = p => p === 1 ? 'In Recommended' : p === 2 ? 'In Competitive' : 'Manual only';
function rowHTML(t) {
  const s = st(t.id), why = (S.states[t.id] || {}).why;
  const on = s === 'on' || s === 'native';
  const busy = S.busy.has(t.id);
  const stateTag = s === 'native' ? `<span class="state-native">Already set on this PC</span>`
    : s === 'reset' ? `<span class="state-reset">${esc(why || 'Changed back since you applied it')}</span>`
    : s === 'na' ? `<span class="state-na">${esc(why || 'Not available')}</span>` : '';
  const lk = locked(t);
  return `
  <div class="row${s === 'na' ? ' na' : ''}${lk ? ' locked' : ''}" data-row="${t.id}">
    ${lk ? `<button class="prolock" data-act="upgrade" type="button" aria-label="Unlock with a key" title="Unlock with a key">${ico('lock')}</button>`
      : `<button class="switch${s === 'native' ? ' native' : ''}${busy ? ' busy' : ''}" role="switch" aria-checked="${on}" aria-label="${esc(t.title)}" data-act="toggle" data-id="${t.id}" type="button"${s === 'na' || busy ? ' disabled' : ''}></button>`}
    <div class="title">${esc(t.title)}</div>
    <div>${t.reboot ? `<span title="Needs a restart to take effect">${ico('reboot', 'reboot-ico')}</span>` : ''}</div>
    <div class="desc">${esc(t.desc)}</div>
    <div class="benefit">${ico('spark')}<span>${esc(t.benefit)}</span></div>
    ${t.note ? `<div class="note">${ico('warn')}<span>${esc(t.note)}</span></div>` : ''}
    <div class="meta">
      <span><span class="impact l${t.impact}"><i></i><i></i><i></i></span> ${impactWord[t.impact]} impact</span>
      ${t.risk !== 'safe' ? `<span class="risk-${t.risk}">${t.risk === 'expert' ? 'Expert' : 'Moderate risk'}</span>` : '<span>Safe</span>'}
      <span>${profileWord(t.profile)}</span>
      ${t.online ? `<span class="from-pack">${ico('spark')}Live tweak</span>` : ''}
      ${t.pro ? `<span class="pro-tag">${ico('lock')}Key</span>` : ''}
      ${stateTag}
      <button data-act="changes" data-id="${t.id}" type="button">${S.expanded.has(t.id) ? 'Hide changes' : 'What it changes'}</button>
    </div>
    ${S.expanded.has(t.id) ? `<div class="changes">${(t.changes || []).map(c => `<div>${esc(c)}</div>`).join('')}</div>` : ''}
  </div>`;
}

function groupsHTML(list, single) {
  const groups = [];
  list.forEach(t => {
    let g = groups.find(x => x.name === t.group);
    if (!g) { g = { name: t.group, items: [] }; groups.push(g); }
    g.items.push(t);
  });
  return groups.map(g => `
    <div class="group">
      <div class="group-h"><h3>${esc(g.name)}</h3><span>${appliedCount(g.items)} of ${availCount(g.items)} on</span></div>
      <div class="rows${single || g.items.length === 1 ? ' single' : ''}">${g.items.map(rowHTML).join('')}</div>
    </div>`).join('');
}

function viewCategory(c) {
  let list = catTweaks(c.id);
  const a = appliedCount(list), n = availCount(list);
  const pend = list.filter(t => t.profile > 0 && avail(t.id) && !isOn(t.id)).length;
  const undoable = list.filter(t => st(t.id) === 'on' || st(t.id) === 'reset').length;
  if (S.filter === 'off') list = list.filter(t => !isOn(t.id));
  if (S.filter === 'on') list = list.filter(t => isOn(t.id));
  return `
    ${categoryIntro(c.id)}
    <div class="toolbar">
      <div class="seg" role="tablist">
        ${[['all', 'All'], ['off', 'Off'], ['on', 'On']].map(([k, l]) => `<button class="${S.filter === k ? 'on' : ''}" data-filter="${k}" type="button">${l}</button>`).join('')}
      </div>
      <span class="sum"><b>${a}</b> of ${n} on</span>
      <span class="spacer"></span>
      <button class="btn sm ghost" data-act="undoCat" data-cat="${c.id}" type="button"${undoable ? '' : ' disabled'}>${ico('undo')}Undo this section</button>
      <button class="btn sm primary" data-act="applyCat" data-cat="${c.id}" type="button"${pend ? '' : ' disabled'}>${ico('check')}Turn on ${pend ? plural(pend, 'suggested tweak') : 'suggested tweaks'}</button>
    </div>
    ${list.length ? groupsHTML(list) : `<div class="empty"><h3>Nothing here</h3><p>No tweaks match this filter.</p></div>`}
    ${c.id === 'appearance' || list.some(t => t.profile === 0) ? `<p class="dim small" style="margin-top:6px">Manual-only tweaks are never applied by Optimize or “Turn on suggested”. Switch them on yourself if you want them.</p>` : ''}`;
}

function categoryIntro(id) {
  const e = S.data.env || {};
  const gc = S.data.categories.find(c => c.id === id);
  if (gc && gc.game) return gameIntro(gc.game);
  if (id === 'apps') {
    return `<div class="intro">
      <div class="panel spot"><div class="panel-h"><h2>Turn off QoS inside Discord</h2></div><div class="panel-b">
        <p class="muted" style="margin:0 0 12px">Discord keeps this switch in its own account settings, which no app can change for you. It takes 20 seconds:</p>
        <ol class="steps"><li>Open Discord and click the <b>gear</b> next to your name (User Settings).</li><li>Go to <b>Voice &amp; Video</b>, then scroll to <b>Quality of Service</b>.</li><li>Turn off <b>Enable Quality of Service High Packet Priority</b>.</li><li>While you're there, turn off <b>Game Overlay</b> under Activity Settings if you don't use it.</li></ol>
        <div class="actions"><button class="btn sm ghost" data-open="discord" type="button">${ico('ext')}Open Discord</button></div>
      </div></div>
      <div class="panel spot"><div class="panel-h"><h2>What the tweaks below do</h2></div><div class="panel-b">
        <p class="muted" style="margin:0">The QoS tweak adds a Windows policy that strips Discord's priority tag at the network level, so voice packets stop competing with Fortnite even if the switch in Discord is still on. Hardware acceleration needs Discord fully closed first, because Discord rewrites its settings file when it quits.</p>
      </div></div></div>`;
  }
  if (id === 'gpu') {
    const nv = (e.vendors || []).includes('nvidia'), amd = (e.vendors || []).includes('amd');
    const nvSteps = `<ol class="steps"><li>Open <b>NVIDIA Control Panel</b> → Manage 3D settings → Global.</li><li><b>Power management mode</b>: Prefer maximum performance.</li><li><b>Low Latency Mode</b>: On (or leave Off and turn on NVIDIA Reflex inside Fortnite).</li><li><b>Shader Cache Size</b>: 10 GB or Unlimited.</li><li><b>Texture filtering – Quality</b>: High performance.</li></ol>`;
    const amdSteps = `<ol class="steps"><li>Open <b>AMD Software</b> → Gaming → Graphics.</li><li>Turn on <b>Radeon Anti-Lag</b>.</li><li>Turn off <b>Radeon Chill</b> and <b>Radeon Boost</b> for consistent frame times.</li><li>Set <b>Texture Filtering Quality</b> to Performance.</li></ol>`;
    return `<div class="intro">
      <div class="panel spot"><div class="panel-h"><h2>${nv ? 'NVIDIA driver settings' : amd ? 'AMD driver settings' : 'Graphics driver settings'}</h2></div><div class="panel-b">
        <p class="muted" style="margin:0 0 12px">Driver panel settings live inside the driver itself, so set these by hand once:</p>
        ${amd && !nv ? amdSteps : nvSteps}
        <div class="actions">${nv || !amd ? `<button class="btn sm ghost" data-open="nvcp" type="button">${ico('ext')}Open NVIDIA Control Panel</button>` : ''}${amd ? `<button class="btn sm ghost" data-open="amd" type="button">${ico('ext')}Open AMD Software</button>` : ''}<button class="btn sm quiet" data-open="graphics" type="button">Windows graphics settings</button></div>
      </div></div>
      <div class="panel spot"><div class="panel-h"><h2>Your graphics cards</h2></div><div class="panel-b">
        <dl class="kv">${(e.gpus || []).map(g => `<dt>${esc(g.vendor ? g.vendor.toUpperCase() : 'GPU')}</dt><dd>${esc(g.name)}${g.driver ? `<span class="dim small"> · driver ${esc(g.driver)}</span>` : ''}</dd>`).join('') || '<dt>GPU</dt><dd class="dim">Not detected</dd>'}</dl>
        <p class="dim small" style="margin:14px 0 0">Keep your driver up to date from NVIDIA, AMD or Intel directly. A clean driver install fixes more stutter than any registry tweak.</p>
      </div></div></div>`;
  }
  if (id === 'kernel') {
    const xmp = e.ramXmp === 'ok' ? `<span class="pill ok">${ico('check')}${e.ramConf} MT/s</span>`
      : e.ramXmp === 'likely_off' ? `<span class="pill warn">${e.ramConf} MT/s · likely default speed</span>`
      : e.ramXmp === 'below_rated' ? `<span class="pill warn">${e.ramConf} of ${e.ramSpeed} MT/s</span>` : `<span class="pill">${e.probed ? 'Unknown' : 'Checking…'}</span>`;
    const sb = e.secureBoot === true ? '<span class="pill ok">On</span>' : e.secureBoot === false ? '<span class="pill">Off</span>' : '<span class="pill">Unknown</span>';
    const hv = e.vbs < 0 ? '<span class="pill">Unknown</span>' : e.hvci ? '<span class="pill warn">On</span>' : '<span class="pill ok">Off</span>';
    const bl = e.bitlocker === 1 ? '<span class="pill warn">On · paused automatically for boot changes</span>' : e.bitlocker === 0 ? '<span class="pill">Off</span>' : '<span class="pill">Unknown</span>';
    return `<div class="intro">
      <div class="panel spot"><div class="panel-h"><h2>BIOS checklist</h2></div><div class="panel-b">
        <p class="muted" style="margin:0 0 12px">No Windows app can safely change BIOS settings. These are worth checking once, by hand:</p>
        <ol class="steps">
          <li><b>XMP / EXPO / DOCP</b>: turn it on so your RAM runs at its rated speed. This is often the single biggest free FPS gain.</li>
          <li><b>Resizable BAR</b> and <b>Above 4G Decoding</b>: on, for modern GPUs.</li>
          <li><b>Precision Boost Overdrive</b> (AMD) or <b>Turbo Boost</b> (Intel): on/auto.</li>
          <li><b>Update your BIOS</b> if it's more than a year old. Updates often fix stutter and memory stability.</li>
        </ol>
        <div class="actions"><button class="btn sm ghost" data-act="bios" type="button">${ico('reboot')}Restart into BIOS</button></div>
      </div></div>
      <div class="panel spot"><div class="panel-h"><h2>Detected on this PC</h2></div><div class="panel-b">
        <dl class="kv"><dt>Memory speed</dt><dd>${xmp}</dd><dt>Firmware</dt><dd>${esc(e.firmware || 'Unknown')}${e.bios ? `<span class="dim small"> · BIOS ${esc(e.bios)}</span>` : ''}</dd>
        <dt>Secure Boot</dt><dd>${sb}</dd><dt>Memory Integrity</dt><dd>${hv}</dd><dt>BitLocker</dt><dd>${bl}</dd></dl>
        <p class="dim small" style="margin:14px 0 0">Boot timer tweaks below edit Windows' boot settings. If BitLocker is on, AstroSpeed pauses it for one restart first so you won't be asked for a recovery key.</p>
      </div></div></div>`;
  }
  if (id === 'input') {
    return `<div class="intro"><div class="panel spot"><div class="panel-h"><h2>Your aim stays exactly the same</h2></div><div class="panel-b">
      <p class="muted" style="margin:0">This section only keeps USB ports and devices awake and stops the Sticky Keys pop-up. Sensitivity, pointer speed, “Enhance pointer precision”, DPI, polling rate and mouse/keyboard driver buffers are never changed by any tweak in AstroSpeed.</p>
    </div></div></div>`;
  }
  return '';
}

function viewSearch() {
  const q = S.search.toLowerCase().trim();
  const list = T().filter(t => (t.title + ' ' + t.desc + ' ' + t.benefit + ' ' + t.group + ' ' + (t.changes || []).join(' ')).toLowerCase().includes(q));
  if (!list.length) return `<div class="empty"><h3>No tweaks found</h3><p>Try a word like “Discord”, “power”, “timer” or “GPU”.</p></div>`;
  return S.data.categories.map(c => {
    const items = list.filter(t => t.cat === c.id);
    if (!items.length) return '';
    return `<div class="group-h" style="margin-top:6px"><h3 style="font-family:Unbounded;font-weight:500">${esc(c.title)}</h3></div>${groupsHTML(items)}`;
  }).join('');
}

// ───────────────────────── tools ─────────────────────────
function viewBooster() {
  const b = S.booster;
  const apps = S.bgApps;
  if (S.bgSel === null && apps) S.bgSel = new Set(apps.filter(a => a.default).map(a => a.name));
  return `
  <div class="tools-grid" style="margin-bottom:20px">
    <div class="panel tool spot">
      <h3>Timer resolution</h3>
      <div class="big">${b ? b.timerCur.toFixed(2) : '–'}<small>ms now</small></div>
      <p class="muted small grow" style="margin:0">Holds Windows' timer at ${b ? b.timerMax.toFixed(1) : '0.5'} ms while AstroSpeed is open, for tighter frame pacing. Released when you close the app.</p>
      <div style="display:flex;align-items:center;gap:12px"><button class="switch" role="switch" aria-checked="${b && b.timerHeld}" data-act="timer" type="button" aria-label="Hold timer resolution"></button><span>${b && b.timerHeld ? 'Holding' : 'Off'}</span></div>
    </div>
    <div class="panel tool spot">
      <h3>Memory</h3>
      <div class="big">${b ? fmtMB(b.freeMB) : '–'}<small>free of ${b ? fmtMB(b.totalMB) : '–'}</small></div>
      ${b ? `<div class="meterbar"><i style="width:${Math.max(3, 100 - b.freeMB / b.totalMB * 100).toFixed(0)}%"></i></div>` : ''}
      <p class="muted small grow" style="margin:0">${b && b.standbyMB >= 0 ? `${fmtMB(b.standbyMB)} is sitting in Windows' standby cache. ` : ''}Clearing it gives your game room to load without Windows evicting pages mid-match.</p>
      <div><button class="btn sm ghost" data-act="purge" type="button">${ico('broom')}Clear standby memory</button></div>
    </div>
    <div class="panel tool spot">
      <h3>Game process</h3>
      <div class="big" style="font-size:24px">${b ? (b.game ? esc(b.game) : 'No game running') : '–'}</div>
      <p class="muted small grow" style="margin:0">Raise the running game to high priority right now. The game tweaks make this automatic on every launch.</p>
      <div style="display:flex;gap:12px;align-items:center;flex-wrap:wrap"><button class="btn sm ghost" data-act="prio" type="button"${b && b.fortnite ? '' : ' disabled'}>${ico('rocket')}Set high priority now</button></div>
    </div>
  </div>
  <div class="panel spot autoboost" style="margin-bottom:20px"><div class="panel-b ab-row">
    <button class="switch" role="switch" aria-checked="${!!S.data.autoBoost}" data-act="autoboost" type="button" aria-label="Boost automatically when a game starts"></button>
    <div><div class="ab-t">Boost automatically when a game starts</div><div class="muted small">While AstroSpeed is open, it watches for any supported game. When one starts, it clears standby memory, tightens the timer and raises the game to high priority. Nothing is closed.</div></div>
  </div></div>
  <div class="panel spot">
    <div class="panel-h"><h2>Background apps</h2><button class="btn sm quiet" data-act="bgrefresh" type="button">${ico('reboot')}Refresh</button></div>
    <div class="panel-b">
      <p class="muted small" style="margin:0 0 6px">Apps that are running right now and can safely be closed before a match. Mouse, keyboard and headset software is never listed, so your DPI and lighting stay put.</p>
      ${apps === null ? '<div class="empty"><div class="spin" style="margin:auto"></div></div>' : apps.length ? `<div class="list">${apps.map(a => `
        <label class="li"><input class="cb" type="checkbox" data-bg="${esc(a.name)}"${S.bgSel.has(a.name) ? ' checked' : ''}><div><div class="n">${esc(a.name)}</div>${a.note ? `<div class="d">${esc(a.note)}</div>` : ''}</div><div class="s">${fmtMB(a.memMB)}</div></label>`).join('')}</div>` : '<div class="empty" style="padding:30px"><p>No known background apps are running. Nice.</p></div>'}
    </div>
    <div class="dialog-f" style="border-top:1px solid var(--line)">
      <span class="muted small">Boost now clears standby memory, holds the timer, flushes DNS, raises your game's priority and closes the apps you ticked.</span>
      <span class="spacer"></span>
      <button class="btn ghost" data-act="bgclose" type="button"${apps && apps.length ? '' : ' disabled'}>Close selected</button>
      <button class="btn primary" data-act="boost" type="button">${ico('rocket')}Boost now</button>
    </div>
  </div>`;
}

function viewCleaner() {
  const res = S.clean;
  if (S.cleanSel === null) S.cleanSel = new Set(S.data.cleanTargets.filter(t => t.default).map(t => t.id));
  const size = id => { const r = res && res.find(x => x.id === id); return r ? r.bytes : null; };
  const total = S.data.cleanTargets.filter(t => S.cleanSel.has(t.id)).reduce((s, t) => s + (size(t.id) || 0), 0);
  return `
  <div class="panel spot">
    <div class="panel-h"><h2>What to clean</h2><button class="btn sm quiet" data-act="scan" type="button">${ico('reboot')}Scan again</button></div>
    <div class="panel-b"><div class="list">
      ${S.data.cleanTargets.map(t => `
        <label class="li"><input class="cb" type="checkbox" data-clean="${t.id}"${S.cleanSel.has(t.id) ? ' checked' : ''}>
          <div><div class="n">${esc(t.title)}</div><div class="d">${esc(t.desc)}${t.close ? ` Close ${esc(t.close)} first for best results.` : ''}</div></div>
          <div class="s">${res === null ? '<div class="spin" style="margin-left:auto"></div>' : fmtBytes(size(t.id))}</div></label>`).join('')}
    </div></div>
    <div class="dialog-f">
      <span class="sum">Selected: <b>${res === null ? '…' : fmtBytes(total)}</b></span>
      <span class="muted small">Files in use and anything changed in the last 2 minutes are skipped.</span>
      <span class="spacer"></span>
      <button class="btn primary" data-act="clean" type="button"${res === null || !S.cleanSel.size ? ' disabled' : ''}>${ico('broom')}Clean selected</button>
    </div>
  </div>`;
}

function viewStartup() {
  const items = S.startup;
  if (items === null) return '<div class="empty"><div class="spin" style="margin:auto"></div></div>';
  if (!items.length) return '<div class="empty"><h3>Nothing starts with Windows</h3><p>No startup apps were found.</p></div>';
  const on = items.filter(i => i.enabled).length;
  return `
  <div class="toolbar"><span class="sum"><b>${on}</b> of ${items.length} start with Windows</span><span class="spacer"></span><button class="btn sm quiet" data-open="startup" type="button">${ico('ext')}Windows startup settings</button></div>
  <div class="rows single">${items.map(i => `
    <div class="row" style="grid-template-columns:auto 1fr auto">
      <button class="switch" role="switch" aria-checked="${i.enabled}" data-act="startup" data-id="${esc(i.id)}" type="button" aria-label="${esc(i.name)}"></button>
      <div class="title">${esc(i.name)}</div><div class="dim small">${esc(i.location)} · ${esc(i.scope)}</div>
      <div class="desc cmd" title="${esc(i.command)}">${esc(i.command)}</div>
    </div>`).join('')}</div>
  <p class="dim small">Leave security software, audio drivers and your mouse/keyboard software on.</p>`;
}

function viewNetwork() {
  const p = S.ping;
  const maxAvg = p ? Math.max(...p.filter(x => !x.error).map(x => x.avg), 1) : 1;
  const best = p ? p.filter(x => !x.error).sort((a, b) => a.avg - b.avg)[0] : null;
  return `
  <div class="grid" style="grid-template-columns:1.6fr 1fr;align-items:start">
    <div class="panel spot">
      <div class="panel-h"><h2>Fortnite server ping</h2><button class="btn sm primary" data-act="ping" type="button"${S.pinging ? ' disabled' : ''}>${S.pinging ? '<span class="spin"></span>Testing…' : ico('signal') + 'Run ping test'}</button></div>
      <div class="panel-b">
        ${p ? `${best ? `<p class="muted" style="margin:0 0 10px">Your best region is <b style="color:var(--ink)">${esc(best.name)}</b> at ${Math.round(best.avg)} ms. Pick it under Matchmaking Region in Fortnite's settings.</p>` : ''}
        <table class="ping"><thead><tr><th>Region</th><th></th><th class="num">Ping</th><th class="num">Jitter</th><th class="num">Loss</th></tr></thead><tbody>
        ${p.map((r, i) => `<tr style="--i:${i}"><td>${esc(r.name)}</td><td style="width:40%">${r.error ? `<span class="dim small">${esc(r.error)}</span>` : `<div class="bar"><i style="width:${Math.max(4, r.avg / maxAvg * 100)}%"></i></div>`}</td>
          <td class="num">${r.error ? '–' : Math.round(r.avg) + ' ms'}</td><td class="num">${r.error ? '–' : r.jitter.toFixed(1) + ' ms'}</td><td class="num">${r.loss}%</td></tr>`).join('')}
        </tbody></table>` : `<p class="muted" style="margin:0">Sends a few pings to each Fortnite region's matchmaking endpoint and shows the average, jitter and packet loss. Takes about 5 seconds.</p>`}
      </div>
    </div>
    <div class="panel spot">
      <div class="panel-h"><h2>DNS</h2></div>
      <div class="panel-b">
        <dl class="kv"><dt>Current</dt><dd>${S.dns === null ? '<span class="dim">Checking…</span>' : esc(S.dns || 'Automatic (from your router)')}</dd></dl>
        <p class="muted small" style="margin:12px 0">DNS only affects how fast names are looked up (loading the shop, logging in), not your in-match ping.</p>
        <div class="actions" style="margin-top:0">
          ${[['auto', 'Automatic'], ['cloudflare', 'Cloudflare'], ['google', 'Google'], ['quad9', 'Quad9']].map(([k, l]) => `<button class="btn sm ghost" data-dns="${k}" type="button">${l}</button>`).join('')}
        </div>
        <div class="actions"><button class="btn sm quiet" data-act="flushdns" type="button">${ico('broom')}Flush DNS cache</button></div>
      </div>
    </div>
  </div>
  ${livePingPanel()}`;
}

function viewSafety() {
  const applied = T().filter(t => st(t.id) === 'on' || st(t.id) === 'reset').length;
  const rps = (S.data.restorePoints || []).slice().reverse();
  return `
  <div class="grid" style="grid-template-columns:1fr 1fr;align-items:start">
    <div class="panel spot"><div class="panel-h"><h2>Undo everything</h2></div><div class="panel-b">
      <p class="muted" style="margin:0 0 6px">AstroSpeed saved the exact value of every setting before changing it. Undoing puts each one back, newest first.</p>
      <p style="margin:0 0 16px"><b>${applied}</b> <span class="muted">${applied === 1 ? 'tweak is' : 'tweaks are'} currently applied by AstroSpeed.</span></p>
      <button class="btn danger" data-act="undoAll" type="button"${applied ? '' : ' disabled'}>${ico('undo')}Undo all AstroSpeed changes</button>
    </div></div>
    <div class="panel spot"><div class="panel-h"><h2>System Restore</h2></div><div class="panel-b">
      ${rps.length ? `<div class="list">${rps.slice(0, 5).map(r => `<div class="li" style="grid-template-columns:auto 1fr auto">${ico(r.ok ? 'ok' : 'bad', r.ok ? 'ico-ok' : 'ico-bad')}<div>${esc(r.label)}</div><div class="s">${esc(fmtDate(r.when))}</div></div>`).join('')}</div>` : '<p class="muted" style="margin:0">No restore points created by AstroSpeed yet.</p>'}
      <div class="actions"><button class="btn sm ghost" data-act="restorepoint" type="button">${ico('shield')}Create restore point</button><button class="btn sm quiet" data-open="restore" type="button">${ico('ext')}Open System Restore</button></div>
    </div></div>
  </div>
  <div class="panel spot" style="margin-top:20px"><div class="panel-h"><h2>Activity log</h2><div><button class="btn sm quiet" data-act="logrefresh" type="button">${ico('reboot')}Refresh</button><button class="btn sm quiet" data-open="logs" type="button">${ico('ext')}Open log folder</button></div></div>
    <div class="panel-b"><div class="logbox" id="logbox">${S.log === null ? 'Loading…' : esc((S.log || []).join('\n')) || 'Nothing logged yet.'}</div></div>
  </div>`;
}

function viewAbout() {
  const rec = T().filter(t => t.profile === 1).length, comp = T().filter(t => t.profile === 2).length, man = T().filter(t => t.profile === 0).length;
  return `
  <div class="grid" style="grid-template-columns:1fr 1fr;align-items:start">
    <div class="panel spot"><div class="panel-h"><h2>How AstroSpeed works</h2></div><div class="panel-b">
      <ol class="steps">
        <li>Every tweak reads the current value first and saves it to a backup file before changing anything.</li>
        <li><b>Recommended</b> (${rec} tweaks) is safe on any PC. <b>Competitive</b> adds ${comp} more that trade a little power use or convenience for latency. ${man} tweaks are <b>manual only</b> because they reduce security or change how Windows looks.</li>
        <li>Each switch shows its impact, its risk and the exact registry keys, services or commands it changes.</li>
        <li>Undo puts the saved values back exactly, including deleting keys that didn't exist before.</li>
      </ol>
      <p class="muted small" style="margin:16px 0 0">We leave out popular “tweaks” that don't do anything on modern Windows, such as IRQ8Priority, LargeSystemCache or disabling your page file, and anything that disables Windows Defender, the firewall, UAC or Windows Update.</p>
    </div></div>
    <div class="panel spot"><div class="panel-h"><h2>Details</h2></div><div class="panel-b">
      <dl class="kv"><dt>Version</dt><dd>${esc(S.data.version)}</dd><dt>Tweaks</dt><dd>${S.data.tweakCount} (${S.data.actionCount} individual changes before per-device expansion)</dd>
      <dt>Backups and logs</dt><dd>${esc(S.data.dataDir || 'C:\\ProgramData\\AstroSpeed')}</dd>
      <dt>Results</dt><dd class="muted">Gains depend on your hardware. Older and lower-end PCs usually see the biggest difference.</dd></dl>
      <p class="dim small" style="margin:16px 0 0">Fonts: Unbounded and Instrument Sans, SIL Open Font License. AstroSpeed is not affiliated with or endorsed by Epic Games.</p>
      <div class="actions"><button class="btn sm quiet" data-open="logs" type="button">${ico('ext')}Open data folder</button></div>
    </div></div>
  </div>`;
}

async function afterTool() {
  if (S.page === 'checkup') {
    if (!S.checkup && !S.scanning) runScan();
    else if (S.checkup && S.scanStamp) animateCheckup();
  }
  if (S.page === 'network') drawLivePing(true);
  if (S.page === 'booster') {
    if (!S.booster) api('boostStatus').then(b => { S.booster = b; if (S.page === 'booster') rerenderView(); }).catch(() => {});
    if (S.bgApps === null) api('bgApps').then(a => { S.bgApps = a || []; if (S.page === 'booster') rerenderView(); }).catch(() => { S.bgApps = []; });
  }
  if (S.page === 'cleaner' && S.clean === null) api('cleanScan').then(r => { S.clean = r || []; if (S.page === 'cleaner') rerenderView(); }).catch(e => toast(e.message, 'err'));
  if (S.page === 'startup' && S.startup === null) api('startupList').then(r => { S.startup = r || []; if (S.page === 'startup') rerenderView(); }).catch(e => toast(e.message, 'err'));
  if (S.page === 'network' && S.dns === null) api('dnsGet').then(r => { S.dns = r || ''; if (S.page === 'network') rerenderView(); }).catch(() => { S.dns = ''; });
  if (S.page === 'settings' && S.startupOn === undefined) api('startupGet').then(r => { S.startupOn = !!r; if (S.page === 'settings') rerenderView(); }).catch(() => { S.startupOn = false; });
  if (S.page === 'safety' && S.log === null) api('log').then(r => { S.log = r || []; if (S.page === 'safety') { rerenderView(); const lb = $('#logbox'); if (lb) lb.scrollTop = lb.scrollHeight; } }).catch(() => { S.log = []; });
}
function rerenderView() {
  const v = $('#view'), top = v.scrollTop;
  render();
  v.scrollTop = top;
}

// ───────────────────────── toasts & modal ─────────────────────────
function toast(msg, kind = 'info', ms = 4200) {
  const el = document.createElement('div');
  el.className = 'toast ' + kind;
  el.innerHTML = ico(kind === 'ok' ? 'ok' : kind === 'err' ? 'warn' : 'info') + `<span>${esc(msg)}</span>`;
  el.style.setProperty('--life', ms + 'ms');
  $('#toasts').appendChild(el);
  setTimeout(() => { el.classList.add('leave'); setTimeout(() => el.remove(), 380); }, ms);
}
let modalToken = 0;
function openModal(html, narrow) {
  const m = $('#modal');
  const tok = ++modalToken;
  const already = !m.hidden && m.classList.contains('show');
  m.innerHTML = `<div class="dialog${narrow ? ' narrow' : ''}" role="dialog" aria-modal="true">${html}</div>`;
  m.hidden = false;
  if (!already) { m.classList.remove('show'); void m.offsetWidth; }
  requestAnimationFrame(() => { if (tok === modalToken) m.classList.add('show'); });
  const f = m.querySelector('[data-autofocus]') || m.querySelector('button');
  if (f) f.focus({ preventScroll: true });
}
function closeModal() {
  const m = $('#modal');
  if (m.dataset.locked || m.hidden) return;
  const tok = ++modalToken;
  m.classList.remove('show');
  setTimeout(() => { if (tok === modalToken) { m.hidden = true; m.innerHTML = ''; } }, 320);
}
function confirmBox(title, body, okLabel, danger) {
  return new Promise(res => {
    openModal(`<div class="dialog-h"><div><h2>${esc(title)}</h2><p>${body}</p></div></div>
      <div class="dialog-f"><span class="spacer"></span><button class="btn ghost" data-c="0" type="button">Cancel</button><button class="btn ${danger ? 'danger' : 'primary'}" data-c="1" data-autofocus type="button">${esc(okLabel)}</button></div>`, true);
    $('#modal').onclick = e => {
      const b = e.target.closest('[data-c]');
      if (b) { $('#modal').onclick = null; closeModal(); res(b.dataset.c === '1'); }
      else if (e.target.id === 'modal') { $('#modal').onclick = null; closeModal(); res(false); }
    };
  });
}

// ───────────────────────── jobs (apply / undo with progress) ─────────────────────────
let profileChoice = 2;
function openOptimize() {
  profileChoice = S.data.profile || 2;
  const rec = pendingFor(1), comp = pendingFor(2);
  const card = (p, name, list, bullets) => `
    <button class="profile${profileChoice === p ? ' on' : ''}${p === 2 && !proUnlocked() ? ' prof-locked' : ''}" data-profile="${p}" type="button">
      <div class="p-name">${esc(name)}${p === 2 && !proUnlocked() ? ` <span class="prof-lock">${ico('lock')}Key</span>` : ''}<span class="p-count">${list.length ? plural(list.length, 'tweak') + ' to apply' : 'All applied'}</span></div>
      <ul>${bullets.map(b => `<li>${esc(b)}</li>`).join('')}</ul>
    </button>`;
  openModal(`
    <div class="dialog-h"><div><h2>Optimize your PC</h2><p>Choose how far to go. You can undo any of it later from Restore & undo.</p></div><button class="btn quiet x" data-close type="button" aria-label="Close">${ico('x')}</button></div>
    <div class="dialog-b">
      <div class="profiles">
        ${card(1, 'Recommended', rec, ['Safe on any PC, desktop or laptop', 'Power plan, Game DVR off, network power saving off, Discord QoS, telemetry and background noise', 'Nothing that changes how Windows looks or works day to day'])}
        ${card(2, 'Competitive', comp, ['Everything in Recommended', 'Adds high priority for your games, boot timer tweaks, exclusive fullscreen, interrupt moderation off, memory and scheduler tuning', 'Uses a bit more power at idle; a few take effect after a restart'])}
      </div>
      <label class="optline"><input class="cb" type="checkbox" id="optRestore" checked>Create a System Restore point first (recommended, takes about a minute)</label>
      <p class="dim small" style="margin:10px 0 0">Manual-only tweaks (security trade-offs and appearance) are never included. Your sensitivity and game settings are never touched.</p>
    </div>
    <div class="dialog-f"><span class="muted small" id="optSummary"></span><span class="spacer"></span><button class="btn ghost" data-close type="button">Cancel</button><button class="btn primary lg" data-act="startOptimize" data-autofocus type="button">${ico('glider')}Start optimizing</button></div>`);
  updateOptSummary();
}
function updateOptSummary() {
  const n = pendingFor(profileChoice).length;
  const s = $('#optSummary');
  if (s) s.textContent = n ? `${plural(n, 'tweak')} will be applied.` : 'Everything in this profile is already on.';
  $$('.profile').forEach(b => b.classList.toggle('on', +b.dataset.profile === profileChoice));
}

function runJob(kind, args, title, intro) {
  const m = $('#modal');
  m.dataset.locked = '1';
  openModal(`
    <div class="dialog-h"><div><h2>${esc(title)}</h2><p id="jobNow">${esc(intro)}</p></div></div>
    <div class="dialog-b"><div class="progress"><i id="jobBar"></i></div><div class="flight" id="flight"></div></div>
    <div class="dialog-f" id="jobFoot"><span class="muted small">Keep AstroSpeed open until this finishes.</span></div>`);
  const flight = $('#flight');
  const icon = s => s === 'ok' || s === 'reverted' ? ico('ok', 'ico-ok') : s === 'already' ? ico('ok', 'ico-na') : s === 'partial' ? ico('warn', 'ico-warn') : s === 'failed' ? ico('bad', 'ico-bad') : s === 'locked' ? ico('lock', 'ico-warn') : ico('dash', 'ico-na');
  const word = { ok: 'Applied', reverted: 'Undone', already: 'Already set', skipped: 'Not on this PC', partial: 'Partly applied', failed: 'Failed', nothing: 'Nothing to undo', locked: 'Needs a key' };
  const off = on('progress', p => {
    if (p.phase === 'restore') { $('#jobNow').textContent = p.title + '…'; flight.insertAdjacentHTML('beforeend', `<div class="fl" id="flRestore"><div class="spin"></div><div>${esc(p.title)}</div><div class="m">This can take a minute</div></div>`); }
    if (p.phase === 'restore-ok' || p.phase === 'restore-failed') {
      const r = $('#flRestore'); if (r) r.outerHTML = `<div class="fl">${p.phase === 'restore-ok' ? ico('ok', 'ico-ok') : ico('warn', 'ico-warn')}<div>${esc(p.title)}</div><div class="m">${esc(p.msg || '')}</div></div>`;
    }
    if (p.phase === 'start') {
      $('#jobNow').textContent = p.title;
      flight.insertAdjacentHTML('beforeend', `<div class="fl" data-fl="${p.id}"><div class="spin"></div><div>${esc(p.title)}</div><div class="m"></div></div>`);
      flight.scrollTop = flight.scrollHeight;
    }
    if (p.phase === 'done') {
      const row = flight.querySelector(`[data-fl="${p.id}"]`);
      if (row) {
        row.firstElementChild.outerHTML = icon(p.status);
        row.lastElementChild.textContent = (word[p.status] || p.status) + (p.msg && p.status !== 'already' && p.status !== 'skipped' ? ' · ' + p.msg : (p.status === 'skipped' && p.msg ? ' · ' + p.msg : ''));
        if (p.status === 'failed' || p.status === 'partial') row.lastElementChild.title = p.msg || '';
      }
      $('#jobBar').style.width = (p.n ? p.i / p.n * 100 : 100) + '%';
    }
  });
  return api(kind, args).then(res => {
    off();
    S.bcdReady = true;
    delete m.dataset.locked;
    const r = res.results || [];
    const good = r.filter(x => x.status === 'ok' || x.status === 'reverted').length;
    const bad = r.filter(x => x.status === 'failed' || x.status === 'partial').length;
    const skip = r.filter(x => x.status === 'skipped' || x.status === 'already' || x.status === 'nothing').length;
    $('#jobBar').style.width = '100%';
    $('.progress').classList.add('done');
    $('#jobNow').textContent = kind === 'apply' ? 'Done. Every change is backed up and can be undone.' : 'Undone. Original values are back.';
    flight.insertAdjacentHTML('beforebegin', `<div class="result-head"><svg class="done-ring" viewBox="0 0 64 64" aria-hidden="true"><circle cx="32" cy="32" r="30"/><path d="m20 33 8 8 16-17"/></svg>
      <div class="result-stats"><div><div class="v" data-count="${good}">0</div><div class="l">${kind === 'apply' ? 'tweaks applied' : 'tweaks undone'}</div></div><div><div class="v" data-count="${res.changes || 0}">0</div><div class="l">individual changes</div></div><div><div class="v" data-count="${skip}">0</div><div class="l">already set or not on this PC</div></div>${bad ? `<div><div class="v" style="color:var(--flare)" data-count="${bad}">0</div><div class="l">need attention</div></div>` : ''}</div></div>`);
    $$('.result-stats [data-count]').forEach((el, i) => countUp(el, +el.dataset.count, 1200, 200 + i * 120));
    S.reboot = !!res.reboot;
    $('#jobFoot').innerHTML = `<span class="muted small">${res.reboot ? 'Some changes take effect after a restart.' : 'All changes are live.'}</span><span class="spacer"></span>
      ${res.reboot ? `<button class="btn ghost" data-act="restart" type="button">${ico('reboot')}Restart now</button>` : ''}<button class="btn primary" data-close data-autofocus type="button">Done</button>`;
    return refreshStates();
  }).catch(e => {
    off();
    delete m.dataset.locked;
    $('#jobNow').textContent = e.message;
    $('#jobFoot').innerHTML = `<span class="spacer"></span><button class="btn primary" data-close type="button">Close</button>`;
  });
}

async function refreshStates() {
  try {
    const r = await api('states', { bcd: !!S.bcdReady });
    S.states = r.states || {};
    S.reboot = !!r.reboot;
  } catch (e) { toast(e.message, 'err'); }
  try {
    const d = await api('init');
    S.data.totalChanges = d.totalChanges; S.data.lastOptimize = d.lastOptimize; S.data.restorePoints = d.restorePoints; S.data.profile = d.profile; S.data.env = d.env;
  } catch (e) { /* keep old */ }
  rerenderView();
}

async function undoAll() {
  const n = T().filter(t => st(t.id) === 'on' || st(t.id) === 'reset').length;
  if (!n) { toast('AstroSpeed has nothing applied right now.', 'info'); return; }
  if (await confirmBox('Undo everything?', `This puts back the original value of every setting AstroSpeed changed (${plural(n, 'tweak')}), newest first. Restart afterwards to finish.`, 'Undo everything', true)) runJob('revertAll', {}, 'Undoing all changes', 'Starting…');
}

async function toggleTweak(id) {
  const t = byId(id), s = st(id);
  if (locked(t)) { showUpgrade(); return; }
  if (s === 'native') { toast('Windows already had this set before AstroSpeed, so there is nothing to undo.', 'info'); return; }
  const turningOn = !(s === 'on');
  if (turningOn && t.risk === 'expert') {
    const ok = await confirmBox(t.title, esc(t.note || 'This is an expert tweak.') + '<br><br>You can undo it at any time.', 'Apply anyway', true);
    if (!ok) return;
  }
  S.busy.add(id); rerenderView();
  try {
    const res = await api(turningOn ? 'apply' : 'revert', { ids: [id] });
    const r = (res.results || [])[0] || {};
    if (r.status === 'failed') toast(`${t.title}: ${r.msg}`, 'err', 7000);
    else if (r.status === 'partial') toast(`${t.title}: partly applied. ${r.msg}`, 'err', 7000);
    else if (r.status === 'skipped') toast(`${t.title}: ${r.msg || 'not on this PC'}`, 'info');
    else if (r.status === 'already') toast(`${t.title} was already set on this PC.`, 'info');
    else toast((turningOn ? 'Applied: ' : 'Undone: ') + t.title + (t.reboot && r.changes ? ' (restart to finish)' : ''), 'ok');
  } catch (e) { toast(e.message, 'err'); }
  S.busy.delete(id);
  S.flash = { id, on: turningOn };
  await refreshStates();
}

// ───────────────────────── events ─────────────────────────
document.addEventListener('click', async e => {
  const nav = e.target.closest('[data-nav]');
  if (nav) { closeModal(); go(nav.dataset.nav); return; }
  const opn = e.target.closest('[data-open]');
  if (opn) { api('open', { target: opn.dataset.open }).catch(err => toast(err.message, 'err')); return; }
  const fl = e.target.closest('[data-filter]');
  if (fl) { S.filter = fl.dataset.filter; rerenderView(); return; }
  if (e.target.closest('[data-close]')) { closeModal(); return; }
  if (e.target.id === 'modal') { closeModal(); return; }
  const pf = e.target.closest('[data-profile]');
  if (pf) { const p = +pf.dataset.profile; if (p === 2 && !proUnlocked()) { showUpgrade(); return; } profileChoice = p; updateOptSummary(); return; }
  const dns = e.target.closest('[data-dns]');
  if (dns) {
    const ok = await confirmBox('Change DNS?', `This changes the DNS server on your active network adapters to <b>${esc(dns.textContent)}</b>. You can switch back to Automatic at any time.`, 'Change DNS');
    if (!ok) return;
    try { await api('dnsSet', { preset: dns.dataset.dns }); S.dns = await api('dnsGet'); toast('DNS updated.', 'ok'); } catch (err) { toast(err.message, 'err'); }
    rerenderView(); return;
  }

  const b = e.target.closest('[data-act]');
  if (!b) return;
  const act = b.dataset.act, id = b.dataset.id;
  switch (act) {
    case 'toggle': toggleTweak(id); break;
    case 'crashDismiss': S.crashSeen = true; renderBanners(); break;
    case 'showUpdate': showUpdate(); break;
    case 'installUpdate': await runUpdate(); break;
    case 'skipUpdate': try { localStorage.setItem('sg-skip-version', S.update.latest); } catch (err) { /* ignore */ } closeModal(); toast(`You won't be reminded about ${S.update.latest}. Settings › Updates still shows it.`, 'info'); break;
    case 'refreshPack': S.packChecking = true; rerenderView(); try { const info = await api('refreshPack'); S.data.pack = info; } catch (err) { S.data.pack = Object.assign({}, S.data.pack, { error: err.message }); } S.packChecking = false; await reloadCatalog(S.data.pack); toast(S.data.pack.error ? S.data.pack.error : 'Live tweaks are up to date.', S.data.pack.error ? 'err' : 'ok'); break;
    case 'msgDismiss': prefSet('sg-msg-seen', (S.data.pack || {}).message || ''); renderBanners(); break;
    case 'checkUpdate': S.checking = true; rerenderView(); try { S.update = await api('checkUpdate'); } catch (err) { S.update = { error: err.message }; } S.checking = false; rerenderView(); if (S.update && S.update.available) showUpdate(); break;
    case 'theme': applyTheme(b.dataset.v); rerenderView(); break;
    case 'setting': {
      const key = b.dataset.key;
      if (key === 'motion') { applyMotion(document.documentElement.classList.contains('motion-off'), true); rerenderView(); break; }
      if ((key === 'autoboost' || key === 'hotkey') && !proUnlocked()) { showUpgrade(); break; }
      if (key === 'autoboost') { try { S.data.autoBoost = await api('setAutoBoost', { on: !S.data.autoBoost }); } catch (err) { toast(err.message, 'err'); } rerenderView(); break; }
      const cur = key === 'startup' ? !!S.startupOn : !!S.data[key];
      try {
        await api('setSetting', { key, on: !cur });
        if (key === 'startup') S.startupOn = !cur; else S.data[key] = !cur;
        toast({ startup: !cur ? 'AstroSpeed will start with Windows.' : 'AstroSpeed won’t start with Windows.', hotkey: !cur ? 'Hotkey on. Press Ctrl + Alt + B to boost.' : 'Hotkey off.', autoUpdate: !cur ? 'AstroSpeed will check for updates when it opens.' : 'Automatic update checks are off.' }[key], 'ok');
      } catch (err) { toast(err.message, 'err', 7000); }
      rerenderView(); break;
    }
    case 'crashDetails': openModal(`<div class="dialog-h"><div><h2>What happened last time</h2><p>The end of the crash log is below. It's also saved in the logs folder.</p></div><button class="btn quiet x" data-close type="button" aria-label="Close">${ico('x')}</button></div>
      <div class="dialog-b"><div class="logbox">${esc(S.data.lastCrash)}</div></div>
      <div class="dialog-f"><span class="spacer"></span><button class="btn ghost" data-open="logs" type="button">${ico('ext')}Open log folder</button><button class="btn primary" data-close data-autofocus type="button">Close</button></div>`); S.crashSeen = true; renderBanners(); break;
    case 'changes': S.expanded.has(id) ? S.expanded.delete(id) : S.expanded.add(id); rerenderView(); break;
    case 'optimize': openOptimize(); break;
    case 'upgrade': showUpgrade(); break;
    case 'activateKey': await doActivate(); break;
    case 'buyKey': api('open', { target: BUY_URL }).catch(() => {}); break;
    case 'startOptimize': {
      const ids = pendingFor(profileChoice).map(t => t.id);
      const restore = $('#optRestore').checked;
      if (!ids.length && !restore) { closeModal(); toast('Everything in this profile is already on.', 'info'); return; }
      api('setProfile', { profile: profileChoice }).catch(() => {});
      runJob('apply', { ids, restore, profile: profileChoice, label: 'AstroSpeed Tweaks - before optimize' }, 'Optimizing', 'Starting…');
      break;
    }
    case 'apply1': S.busy.add(id); rerenderView(); try { await api('apply', { ids: [id] }); toast('Applied: ' + byId(id).title, 'ok'); } catch (err) { toast(err.message, 'err'); } S.busy.delete(id); refreshStates(); break;
    case 'applyCat': {
      const ids = catTweaks(b.dataset.cat).filter(t => t.profile > 0 && avail(t.id) && !isOn(t.id)).map(t => t.id);
      if (ids.length) runJob('apply', { ids }, 'Applying ' + S.data.categories.find(c => c.id === b.dataset.cat).title, 'Starting…');
      break;
    }
    case 'undoCat': {
      const ids = catTweaks(b.dataset.cat).filter(t => st(t.id) === 'on' || st(t.id) === 'reset').map(t => t.id);
      if (!ids.length) return;
      if (await confirmBox('Undo this section?', `This puts back the original values for ${plural(ids.length, 'tweak')} in this section.`, 'Undo section')) runJob('revert', { ids }, 'Undoing', 'Starting…');
      break;
    }
    case 'undoAll': undoAll(); break;
    case 'restart': if (await confirmBox('Restart now?', 'Save anything you have open. Your PC will restart in 5 seconds.', 'Restart')) api('restart', { firmware: false }).catch(err => toast(err.message, 'err')); break;
    case 'bios': if (await confirmBox('Restart into BIOS?', 'Your PC will restart straight into its BIOS/UEFI setup in 5 seconds. Save anything you have open. To leave without changes, choose <b>Exit without saving</b>.', 'Restart into BIOS')) api('restart', { firmware: true }).catch(err => toast(err.message, 'err', 8000)); break;
    case 'restorepoint': {
      toast('Creating a restore point. This can take a minute…', 'info');
      try { await api('restorePoint'); toast('Restore point created.', 'ok'); } catch (err) { toast('Restore point failed: ' + err.message, 'err', 8000); }
      refreshStates(); break;
    }
    case 'reprobe': toast('Rescanning your PC…', 'info'); try { await api('env'); const env = await api('probe', { force: true }); S.data.env = env; } catch (err) { toast(err.message, 'err'); } refreshStates(); break;
    case 'timer': try { await api('boostTimer', { on: !(S.booster && S.booster.timerHeld) }); S.booster = await api('boostStatus'); } catch (err) { toast(err.message, 'err'); } rerenderView(); break;
    case 'purge': try { const mb = await api('boostPurge'); toast(`Standby memory cleared. ${fmtMB(Math.max(0, mb))} more is free.`, 'ok'); S.booster = await api('boostStatus'); } catch (err) { toast(err.message, 'err'); } rerenderView(); break;
    case 'prio': try { const r = await api('boostPriority'); toast(r.running ? 'Your game is now running at high priority.' : 'No supported game is running.', r.running ? 'ok' : 'info'); } catch (err) { toast(err.message, 'err'); } break;
    case 'autoboost': try { S.data.autoBoost = await api('setAutoBoost', { on: !S.data.autoBoost }); toast(S.data.autoBoost ? 'Auto-boost is on. Games are boosted as they start.' : 'Auto-boost is off.', 'ok'); } catch (err) { toast(err.message, 'err'); } rerenderView(); break;
    case 'rescanGames': toast('Looking for games…', 'info', 2500); try { S.data.env.games = await api('rescanGames'); S.icons = await api('gameIcons'); } catch (err) { toast(err.message, 'err'); } await refreshStates(); toast(plural(gamesFound().length, 'game') + ' found.', 'ok'); break;
    case 'applyGames': { const ids = gamePending().map(t => t.id); if (ids.length) runJob('apply', { ids }, 'Applying game tweaks', 'Starting…'); break; }
    case 'bgrefresh': S.bgApps = null; S.bgSel = null; S.booster = null; rerenderView(); break;
    case 'bgclose': {
      const names = [...S.bgSel];
      if (!names.length) { toast('Tick the apps you want to close first.', 'info'); return; }
      try { const r = await api('bgClose', { names }); toast(`Closed ${plural(r.closed, 'process', 'processes')}, about ${fmtMB(r.freedMB)} freed.`, 'ok'); } catch (err) { toast(err.message, 'err'); }
      S.bgApps = null; S.bgSel = null; rerenderView(); break;
    }
    case 'boost': {
      b.disabled = true;
      const out = [];
      try { await api('boostTimer', { on: true }); out.push('timer held'); } catch (err) { /* ignore */ }
      try { await api('boostPurge'); out.push('memory cleared'); } catch (err) { /* ignore */ }
      try { await api('flushDNS'); out.push('DNS flushed'); } catch (err) { /* ignore */ }
      try { const r = await api('boostPriority'); if (r.running) out.push('game at high priority'); } catch (err) { /* ignore */ }
      if (S.bgSel && S.bgSel.size) { try { const r = await api('bgClose', { names: [...S.bgSel] }); if (r.closed) out.push(plural(r.closed, 'app process', 'app processes') + ' closed'); } catch (err) { /* ignore */ } }
      toast('Boosted: ' + (out.join(', ') || 'nothing to do') + '.', 'ok', 6000);
      S.booster = null; S.bgApps = null; S.bgSel = null; rerenderView(); break;
    }
    case 'scan': S.clean = null; rerenderView(); break;
    case 'clean': {
      const ids = [...S.cleanSel];
      if (ids.includes('recycle') && !(await confirmBox('Empty the Recycle Bin?', 'Files in the Recycle Bin will be permanently deleted.', 'Empty and clean', true))) return;
      b.disabled = true; b.innerHTML = '<span class="spin"></span>Cleaning…';
      try { const r = await api('cleanRun', { ids }); const tot = (r || []).reduce((s, x) => s + x.bytes, 0); toast(`Cleaned ${fmtBytes(tot)}.`, 'ok'); } catch (err) { toast(err.message, 'err'); }
      S.clean = null; rerenderView(); break;
    }
    case 'startup': {
      const item = S.startup.find(i => i.id === id);
      try { await api('startupSet', { id, enabled: !item.enabled }); item.enabled = !item.enabled; toast(`${item.name} will ${item.enabled ? '' : 'no longer '}start with Windows.`, 'ok'); } catch (err) { toast(err.message, 'err'); }
      rerenderView(); break;
    }
    case 'rescan': runScan(); break;
    case 'fixAll': { const ids = [...new Set(checkupItems().filter(i => !i.ok && i.tweak).map(i => i.tweak))].filter(id => byId(id) && st(id) !== 'na'); if (ids.length) { await runJob('apply', { ids }, 'Fixing checkup issues', 'Starting…'); try { S.checkup = await api('checkup'); } catch (err) { /* keep */ } if (S.page === 'checkup') rerenderView(); } break; }
    case 'livePing': { const L = S.livePing; if (L.running) { L.running = false; rerenderView(); } else { L.running = true; L.samples = []; rerenderView(); livePingLoop(); } break; }
    case 'ping': S.pinging = true; rerenderView(); try { S.ping = await api('ping'); } catch (err) { toast(err.message, 'err'); } S.pinging = false; rerenderView(); break;
    case 'flushdns': try { await api('flushDNS'); toast('DNS cache flushed.', 'ok'); } catch (err) { toast(err.message, 'err'); } break;
    case 'logrefresh': S.log = null; rerenderView(); break;
  }
});

document.addEventListener('change', e => {
  const c = e.target;
  if (c.id === 'lpRegion') { S.livePing.region = c.value; S.livePing.samples = []; drawLivePing(); }
  if (c.dataset.bg) { c.checked ? S.bgSel.add(c.dataset.bg) : S.bgSel.delete(c.dataset.bg); }
  if (c.dataset.clean) { c.checked ? S.cleanSel.add(c.dataset.clean) : S.cleanSel.delete(c.dataset.clean); rerenderView(); }
});

let searchTimer;
$('#search').addEventListener('input', e => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    S.search = e.target.value;
    if (S.search.trim()) { S.page = 'search'; render(); }
    else if (S.page === 'search') go('dashboard');
  }, 160);
});
document.addEventListener('keydown', e => {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); $('#search').focus(); $('#search').select(); }
  if (e.key === 'Escape' && !$('#modal').hidden) closeModal();
});
$('#optimizeBtn').addEventListener('click', openOptimize);
$('#undoAllBtn').addEventListener('click', undoAll);
document.addEventListener('contextmenu', e => { if (!e.target.closest('input,.logbox,.changes')) e.preventDefault(); });
let rz;
window.addEventListener('resize', () => {
  clearTimeout(rz);
  rz = setTimeout(() => { if (S.page === 'dashboard' && S.heroDone) { drawContours(); placeGliderAt(setupFlight(), dashNumbers().frac); } moveNavIndicator(); }, 80);
});

// ───────────────────────── boot ─────────────────────────
$('#view').addEventListener('scroll', updateScrollBar, { passive: true });
on('hotkeyboost', d => toast(`Boosted${d.game ? ' ' + d.game : ''}: ${Math.max(0, Math.round(d.freedMB))} MB memory freed, timer tightened.`, 'ok', 5000));
on('packUpdated', info => { const had = (S.data && S.data.pack) || {}; reloadCatalog(info).then(() => { if (info.tweaks > (had.tweaks || 0) || info.games > (had.games || 0)) toast('New live tweaks just arrived.', 'ok', 5000); }); });
on('autoboost', d => toast(`${d.game} started. Boosted: memory cleared, timer tightened, high priority.`, 'ok', 6000));
$('#themeBtn').addEventListener('click', () => applyTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark'));
$('#motionBtn').addEventListener('click', () => { applyMotion(document.documentElement.classList.contains('motion-off'), true); if (S.data) rerenderView(); });

(async function boot() {
  const t0 = performance.now();
  loadPrefs();
  // the native bridge is injected by WebView2; give it a moment on slow machines
  for (let i = 0; i < 100 && !(window.sgInvoke || window.__sgMock); i++) await new Promise(r => setTimeout(r, 50));
  try {
    S.data = await api('init');
    const r = await api('states', { bcd: false });
    S.states = r.states || {}; S.reboot = !!r.reboot;
    try { S.data.env.games = await api('games'); } catch (e) { /* optional */ }
    render();
    api('gameIcons').then(ic => { S.icons = ic || {}; if (S.page === 'games' || S.page === 'dashboard' || isGameCat(S.page)) rerenderView(); }).catch(() => {});
    api('ready').catch(() => {});
  } catch (e) {
    $('#view').innerHTML = `<div class="empty"><h3>AstroSpeed couldn't start</h3><p>${esc(e.message)}</p></div>`;
  }
  const wait = motionOK() ? Math.max(0, 1900 - (performance.now() - t0)) : 0;
  setTimeout(() => {
    $('#splash').classList.add('out');
    $('#app').classList.add('ready');
    S.splashGone = true;
    if (S.data) { S.anim = true; render(); }
    if (S.data) { api('drives').then(d => { S.drives = d || []; if (S.page === 'dashboard') rerenderView(); }).catch(() => {}); setTimeout(autoCheckUpdate, 1200); }
    setTimeout(() => $('#splash').remove(), 1400);
  }, wait);
})();
})();
