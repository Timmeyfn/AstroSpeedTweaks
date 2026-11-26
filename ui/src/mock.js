// Browser-only stand-in for the Go backend, used to preview the UI. Not shipped in the exe.
(() => {
  const C = window.__CATALOG;
const PACK_GAMES = [{ id: 'halo', name: 'Halo Infinite', studio: '343 Industries', exes: ['HaloInfinite.exe'], color: '#3A7D44', online: true }];
let LICD = /#licensed/.test(location.hash);
const FILL = 'lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore'.split(' ');
const scr = (txt, seed) => String(txt || '').split(' ').map((w, i) => { let o = ''; const f = FILL[(i + seed) % FILL.length]; for (let k = 0; k < w.length; k++) o += f[k % f.length]; return o; }).join(' ');
const redactMock = (t, i) => t.pro ? Object.assign({}, t, { title: scr(t.title, i), desc: scr(t.desc, i + 3), benefit: scr(t.benefit, i + 5), note: '', changes: [], redacted: true }) : t;
let TRIAL_END = /trialend/.test(location.hash);
const PACK = () => ({ configured: true, repo: 'your-name/astrospeed-tweaks', found: true, tweaks: 3, games: 1, sections: 1, message: 'New: Halo Infinite support and two network tweaks.', problems: ['my_tweaks.bat: line 14 skipped (inside an if/for block)'], images: {}, checked: new Date().toISOString(), error: '' });
if (C && !C.__licensed) { C.__licensed = true; C.catalog.forEach(t => { t.pro = !['gamemode','gamedvr','telemetry'].includes(t.id); }); }
if (C && !C.__packed) {
  C.__packed = true;
  C.categories.push({ id: 'online', title: 'Online tweaks', nav: 'Online', subtitle: 'Tweaks added from your GitHub repository. They update by themselves when you change the repository.', icon: 'spark' });
  C.categories.push({ id: 'game-halo', title: 'Halo Infinite', subtitle: 'Priority, GPU choice, fullscreen and network marking for Halo Infinite only.', icon: 'gamepad', game: 'halo' });
  const mk = (id, cat, title, desc, group, changes) => ({ id, cat, group, title, desc, benefit: 'Your own tweak, applied with the rest of Optimize.', impact: 2, risk: 'moderate', profile: 2, reboot: false, online: true, actions: [], changes });
  C.catalog.push(mk('bat-my-network', 'online', 'My network', 'Lowers network latency a bit', 'From my_network.bat', ['Set HKLM\\SOFTWARE\\X\\Y = 1', 'Run: netsh int tcp set global ecncapability=disabled']));
  C.catalog.push(mk('nagle-off', 'online', 'Turn off Nagle on every adapter', 'Sends small game packets straight away instead of bundling them.', 'From your GitHub pack', ['Set TcpAckFrequency = 1']));
  C.catalog.push(Object.assign(mk('halo-gpu', 'game-halo', 'Always use your high-performance GPU for Halo Infinite', 'Tells Windows to run Halo Infinite on your dedicated graphics card.', 'Process', []), { risk: 'safe', profile: 1, when: ['game:halo'] }));
}
  const states = {};
  let rnd = 7;
  const r = () => (rnd = (rnd * 9301 + 49297) % 233280) / 233280;
  C.catalog.forEach(t => {
    const x = r();
    states[t.id] = { s: t.profile === 1 ? (x < 0.45 ? 'on' : x < 0.6 ? 'native' : 'off') : t.profile === 2 ? (x < 0.2 ? 'on' : 'off') : 'off' };
  });
  states['amd-ulps'] = { s: 'na', why: 'Needs an AMD GPU' };
  const foundGames = { fortnite: 'C:\\Program Files\\Epic Games\\Fortnite\\FortniteGame\\Binaries\\Win64\\FortniteClient-Win64-Shipping.exe', valorant: 'C:\\Riot Games\\VALORANT\\live\\ShooterGame\\Binaries\\Win64\\VALORANT-Win64-Shipping.exe', cs2: 'D:\\SteamLibrary\\steamapps\\common\\Counter-Strike Global Offensive\\game\\bin\\win64\\cs2.exe', rust: 'D:\\SteamLibrary\\steamapps\\common\\Rust\\RustClient.exe', roblox: 'C:\\Users\\you\\AppData\\Local\\Roblox\\Versions\\version-1\\RobloxPlayerBeta.exe' };
  C.catalog.forEach(t => { (t.when || []).forEach(w => { if (w.startsWith('game:') && !foundGames[w.slice(5)]) states[t.id] = { s: 'na', why: 'Not found on this PC' }; }); });
  states['svc-fax'] = { s: 'na', why: 'Not present on this PC' };
  states['dynamic-tick'] = { s: 'reset', why: 'Something changed this back since it was applied' };
  const env = {
    osName: 'Windows 11 Home', osVersion: '24H2 (build 26100.6584)', build: 26100, cpu: 'AMD Ryzen 5 7600X 6-Core Processor', threads: 12, ramGB: 31.2,
    gpus: [{ name: 'NVIDIA GeForce RTX 4060 Ti', vendor: 'nvidia', vramGB: 8, driver: '32.0.15.8157', driverDate: '1-20-2026' }], vendors: ['nvidia'],
    fortniteDir: 'C:\\Program Files\\Epic Games\\Fortnite', fortniteExe: 'x', games: foundGames, discord: true, admin: true, probed: true, bitlocker: 0,
    ramSpeed: 6000, ramConf: 4800, ramType: 'DDR5', ramXmp: 'likely_off', board: 'Gigabyte B650 GAMING X AX', bios: 'F30', firmware: 'UEFI', secureBoot: true, vbs: 2, hvci: true
  };
  const cleanTargets = window.__CLEAN || [];
  const handlers = {
    games: () => foundGames, rescanGames: () => foundGames, gameIcons: () => ({}),
    live: () => ({ cpu: 18 + Math.random() * 22, ramPct: 41 + Math.random() * 4, ramUsedGB: 13.2, ramGB: 31.2, timer: 0.5, procs: 214, uptimeH: 5.2, game: 'Fortnite' }),
    setAutoBoost: a => a.on,
    drives: () => [{ letter: 'C:', label: 'Windows', freeGB: 212.4, sizeGB: 931 }, { letter: 'D:', label: 'Games', freeGB: 71.2, sizeGB: 1863 }],
    checkup: () => ({ displays: [{ name: 'Display 1', gpu: 'NVIDIA GeForce RTX 4070', width: 2560, height: 1440, hz: 60, maxHz: 165, primary: true }, { name: 'Display 2', gpu: 'NVIDIA GeForce RTX 4070', width: 1920, height: 1080, hz: 60, maxHz: 60, primary: false }], powerPlan: 'Balanced', uptimeHours: 131, memLoad: 64 }),
    pingOnce: () => (Math.random() < 0.04 ? { error: 'No reply', loss: 100 } : { avg: 28 + Math.random() * 9 + (Math.random() < 0.08 ? 40 : 0), loss: 0 }),
    licenseStatus: () => ({ required: true, activated: LICD }),
    trialReverted: () => { TRIAL_END = false; return { required: true, activated: false, key: '' }; },
    shopInfo: () => ({ priceShort: '$20', priceLabel: '$20 AUD', buyEnabled: true }),
    benchStart: () => { if (!window.__benchT) { let t = 0; window.__benchT = setInterval(() => { t++; const base = 190 + Math.sin(t / 5) * 18; const frames = Array.from({ length: Math.round(base) }, (_, i) => 1000 / base * (1 + (Math.random() - .5) * .25) * (i % 97 === 0 ? 2.8 : 1)); window.__benchRec && (window.__benchRec.elapsed = Math.min(window.__benchRec.seconds, window.__benchRec.elapsed + 1)); window.__sg.event('benchLive', { fps: Math.round(base), frameMs: 1000 / base, low1: Math.round(base * .68), frames, cpu: 38 + Math.random() * 8, gpu: 96, gpuGame: 92, ramPct: 48, ramUsedGB: 14.9, sensors: { tempC: 66, clockMHz: 2610, powerW: 141, vramUsedGB: 6.1, vramTotalGB: 8 }, pid: 4242, process: 'FortniteClient-Win64-Shipping.exe', game: 'Fortnite', counting: true, rec: window.__benchRec ? Object.assign({}, window.__benchRec) : null }); if (window.__benchRec && window.__benchRec.elapsed >= window.__benchRec.seconds) { const lab = window.__benchRec.label; window.__benchRec = null; const st = lab === 'after' ? { avgFps: 221.4, low1: 161.2, low01: 120.4, stutters: 3 } : { avgFps: 182.3, low1: 118.7, low01: 84.1, stutters: 14 }; window.__sg.event('benchDone', { id: 'b' + Date.now(), when: Date.now(), game: 'Fortnite', process: 'FortniteClient-Win64-Shipping.exe', label: lab, stats: st }); } }, 1000); } return {}; },
    benchLive: () => ({}), benchCancel: () => { window.__benchRec = null; },
    benchRecord: a => { window.__benchRec = { label: a.label, seconds: 3, elapsed: 0 }; },
    benchTargets: () => [{ pid: 4242, process: 'FortniteClient-Win64-Shipping.exe', name: 'Fortnite', fps: 190 }, { pid: 77, process: 'chrome.exe', name: 'chrome', fps: 60 }],
    benchTarget: () => null,
    benchList: () => /#bench/.test(location.hash) ? [{ id: 'b1', when: Date.now() - 3600e3, game: 'Fortnite', label: 'before', stats: { avgFps: 182.3, low1: 118.7, low01: 84.1, stutters: 14 } }, { id: 'b2', when: Date.now() - 600e3, game: 'Fortnite', label: 'after', stats: { avgFps: 221.4, low1: 161.2, low01: 120.4, stutters: 3 } }] : [],
    benchDelete: () => [], benchSaveImage: () => 'C:\\Users\\you\\Pictures\\Astro Tweaks\\Benchmark.png', overlaySet: () => null,
    fnSettings: () => ({ found: true, running: false, backups: 1, refreshHz: 165, current: { mode: 'DirectX 12', fpsCap: 'Unlimited', vsync: 'Off', viewDistance: 'Epic', shadows: 'High', textures: 'High', effects: 'High' } }),
    fnApply: () => ({ found: true, running: false, backups: 2, refreshHz: 165, current: { mode: 'Performance mode', fpsCap: '165 FPS', vsync: 'Off', viewDistance: 'Medium', shadows: 'Low', textures: 'Low', effects: 'Low' } }),
    fnRestore: () => ({ found: true, running: false, backups: 0, refreshHz: 165, current: { mode: 'DirectX 12', fpsCap: 'Unlimited', vsync: 'Off', viewDistance: 'Epic', shadows: 'High', textures: 'High', effects: 'High' } }),
    resetTweaks: () => ['dynamic-tick'],
    removeKey: () => { LICD = false; return { required: true, activated: false }; },
    activate: (a) => { const ok = /ASTRO/i.test((a && a.key) || ''); if (ok) LICD = true; return ok ? { required: true, activated: true, key: 'ASTRO-•••••-•••••-•••••' } : { required: true, activated: false, error: 'That key isn\'t valid (demo).' }; },
    packStatus: () => PACK(),
    refreshPack: () => PACK(),
    downloadUpdate: () => { let p=0; const t=setInterval(()=>{ p+=12; if(p>=100){p=100;clearInterval(t);} try{window.__sg.event('updateProgress',{pct:p,receivedMB:p*0.08,totalMB:8,done:p>=100});}catch(e){} }, 180); return new Promise(r=>setTimeout(()=>r({ok:true}),1700)); },
    applyUpdate: () => ({ ok: true }),
    checkUpdate: () => ({ configured: true, available: true, current: '1.4.0', latest: '1.5.0', title: 'v1.5.0', notes: '## What\'s new\n- Faster game detection\n- New **Settings** page\n- Fixes for Roblox updates', download: 'https://example.com/Astro Tweaks.zip', page: 'https://example.com' }),
    startupGet: () => false, setSetting: a => a.on,
    init: () => ({ version: '1.9.0', server: 'https://astrospeed.dupelication.workers.dev', afterUpdate: false, license: TRIAL_END ? { required: true, activated: false, trial: true, expired: true, needsRevert: true, key: 'ASTRO-•••••-•••••-•••••', problem: 'Your trial has ended.', expiresAt: Date.now() - 1000 } : { required: true, activated: LICD, key: LICD ? 'ASTRO-•••••-•••••-•••••' : '' }, pack: PACK(), games: C.games.concat(PACK_GAMES), autoBoost: false, hotkey: false, autoUpdate: true, updateConfigured: true, autoClean: false, memGuard: false, lastAutoClean: '2026-11-20T09:10:00+08:00', defaultBat: { available: true, asked: false, on: false, updated: /#batupdated/.test(location.hash), title: 'Fortnite optimiser', desc: 'Sets up the Fortnite companion tool and its settings.', undoable: true, commands: ['reg add "HKCU\\System\\GameConfigStore" /v GameDVR_Enabled /t REG_DWORD /d 0 /f', 'start "" "C:\\Program Files\\Companion\\companion.exe" --setup'] }, categories: C.categories, catalog: (LICD ? C.catalog : C.catalog.map(redactMock)), tweakCount: C.tweakCount, actionCount: C.actionCount, env, admin: true, profile: 2, totalChanges: 214, lastOptimize: '2026-10-16T19:42:00+08:00', restorePoints: [], cleanTargets, dataDir: 'C:\\ProgramData\\AstroSpeed' }),
    states: () => ({ states, reboot: false }),
    probe: () => env, env: () => env,
    boostStatus: () => ({ timerHeld: false, timerCur: 15.625, timerMin: 15.625, timerMax: 0.5, standbyMB: 6120, freeMB: 18540, totalMB: 31950, memLoad: 42, fortnite: true, game: 'Fortnite' }),
    bgApps: () => [{ name: 'Microsoft Edge', default: false, note: 'Closes all Edge windows', memMB: 812, count: 9 }, { name: 'Steam', default: true, memMB: 420, count: 4 }, { name: 'OneDrive', default: true, memMB: 96, count: 1 }, { name: 'Discord', default: false, note: 'Leaves your call', memMB: 610, count: 6 }],
    cleanScan: () => cleanTargets.map((t, i) => ({ id: t.id, bytes: [812e6, 233e6, 41e6, 96e6, 120e6, 1.4e9, 980e6, 0, 310e6, 2.1e9, 640e6, 12e6, 0, 88e6, 450e6, 1.2e9][i] || 0, files: 100 })),
    startupList: () => [{ id: 'r0|Discord', name: 'Discord', command: '"C:\\Users\\you\\AppData\\Local\\Discord\\Update.exe" --processStart Discord.exe', location: 'Registry', scope: 'You', enabled: true }, { id: 'r0|Steam', name: 'Steam', command: '"C:\\Program Files (x86)\\Steam\\steam.exe" -silent', location: 'Registry', scope: 'You', enabled: false }, { id: 'r1|SecurityHealth', name: 'SecurityHealth', command: '%windir%\\system32\\SecurityHealthSystray.exe', location: 'Registry', scope: 'All users', enabled: true }],
    dnsGet: () => '192.168.1.1',
    ping: () => [['oce', 'Oceania', 38], ['asia', 'Asia', 96], ['me', 'Middle East', 188], ['eu', 'Europe', 241], ['nae', 'NA East', 236], ['nac', 'NA Central', 214], ['naw', 'NA West', 168], ['br', 'Brazil', 0]].map(([id, name, avg]) => avg ? { id, name, avg, min: avg - 2, max: avg + 6, jitter: 1.4, loss: 0 } : { id, name, error: 'No reply (server may block ping)', loss: 100 }),
    log: () => ['16:20:01  Astro Tweaks 1.0.0 started (admin: true)', '16:20:44  Applying: Turn off Game DVR background recording', '16:20:44      ✓ HKCU\\System\\GameConfigStore → GameDVR_Enabled = 0'],
  };
  window.__sgMock = (method, args, id) => {
    const a = JSON.parse(args);
    if (method === 'apply' || method === 'revert') {
      const ids = a.ids || [];
      let i = 0;
      const step = () => {
        if (i >= ids.length) { window.__sg.done(id, { ok: { results: ids.map(x => ({ id: x, status: method === 'apply' ? 'ok' : 'reverted', changes: 2 })), changes: ids.length * 2, reboot: true } }); return; }
        const t = C.catalog.find(x => x.id === ids[i]);
        window.__sg.event('progress', { phase: 'start', i, n: ids.length, id: t.id, title: t.title });
        states[t.id] = { s: method === 'apply' ? 'on' : 'off' };
        window.__sg.event('progress', { phase: 'done', i: i + 1, n: ids.length, id: t.id, title: t.title, status: method === 'apply' ? (i === 3 ? 'already' : 'ok') : 'reverted', changes: 2 });
        i++; setTimeout(step, 60);
      };
      setTimeout(step, 100);
      return;
    }
    const h = handlers[method];
    setTimeout(() => window.__sg.done(id, { ok: h ? h(a) : true }), 30);
  };
})();
