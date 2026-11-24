// Browser-only stand-in for the Go backend, used to preview the UI. Not shipped in the exe.
(() => {
  const C = window.__CATALOG;
const PACK_GAMES = [{ id: 'halo', name: 'Halo Infinite', studio: '343 Industries', exes: ['HaloInfinite.exe'], color: '#3A7D44', online: true }];
let LICD = false;
const PACK = () => ({ configured: true, repo: 'your-name/astrospeed-tweaks', found: true, tweaks: 3, games: 1, sections: 1, message: 'New: Halo Infinite support and two network tweaks.', problems: ['my_tweaks.bat: line 14 skipped (inside an if/for block)'], images: {}, checked: new Date().toISOString(), error: '' });
if (C && !C.__licensed) { C.__licensed = true; C.catalog.forEach(t => { t.pro = (t.profile === 2) || /^game-/.test(t.cat) || ['kernel','gpu','network','online'].includes(t.cat); }); }
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
    removeKey: () => { LICD = false; return { required: true, activated: false }; },
    activate: (a) => { const ok = /ASTRO/i.test((a && a.key) || ''); if (ok) LICD = true; return ok ? { required: true, activated: true, key: 'ASTRO-•••••-•••••-•••••' } : { required: true, activated: false, error: 'That key isn\'t valid (demo).' }; },
    packStatus: () => PACK(),
    refreshPack: () => PACK(),
    downloadUpdate: () => { let p=0; const t=setInterval(()=>{ p+=12; if(p>=100){p=100;clearInterval(t);} try{window.__sg.event('updateProgress',{pct:p,receivedMB:p*0.08,totalMB:8,done:p>=100});}catch(e){} }, 180); return new Promise(r=>setTimeout(()=>r({ok:true}),1700)); },
    applyUpdate: () => ({ ok: true }),
    checkUpdate: () => ({ configured: true, available: true, current: '1.4.0', latest: '1.5.0', title: 'v1.5.0', notes: '## What\'s new\n- Faster game detection\n- New **Settings** page\n- Fixes for Roblox updates', download: 'https://example.com/AstroSpeed.zip', page: 'https://example.com' }),
    startupGet: () => false, setSetting: a => a.on,
    init: () => ({ version: '1.9.0', server: 'https://astrospeed.dupelication.workers.dev', afterUpdate: false, license: { required: true, activated: LICD, key: LICD ? 'ASTRO-•••••-•••••-•••••' : '' }, pack: PACK(), games: C.games.concat(PACK_GAMES), autoBoost: false, hotkey: false, autoUpdate: true, updateConfigured: true, categories: C.categories, catalog: C.catalog, tweakCount: C.tweakCount, actionCount: C.actionCount, env, admin: true, profile: 2, totalChanges: 214, lastOptimize: '2026-10-16T19:42:00+08:00', restorePoints: [], cleanTargets, dataDir: 'C:\\ProgramData\\AstroSpeed' }),
    states: () => ({ states, reboot: false }),
    probe: () => env, env: () => env,
    boostStatus: () => ({ timerHeld: false, timerCur: 15.625, timerMin: 15.625, timerMax: 0.5, standbyMB: 6120, freeMB: 18540, totalMB: 31950, memLoad: 42, fortnite: true, game: 'Fortnite' }),
    bgApps: () => [{ name: 'Microsoft Edge', default: false, note: 'Closes all Edge windows', memMB: 812, count: 9 }, { name: 'Steam', default: true, memMB: 420, count: 4 }, { name: 'OneDrive', default: true, memMB: 96, count: 1 }, { name: 'Discord', default: false, note: 'Leaves your call', memMB: 610, count: 6 }],
    cleanScan: () => cleanTargets.map((t, i) => ({ id: t.id, bytes: [812e6, 233e6, 41e6, 96e6, 120e6, 1.4e9, 980e6, 0, 310e6, 2.1e9, 640e6, 12e6, 0, 88e6, 450e6, 1.2e9][i] || 0, files: 100 })),
    startupList: () => [{ id: 'r0|Discord', name: 'Discord', command: '"C:\\Users\\you\\AppData\\Local\\Discord\\Update.exe" --processStart Discord.exe', location: 'Registry', scope: 'You', enabled: true }, { id: 'r0|Steam', name: 'Steam', command: '"C:\\Program Files (x86)\\Steam\\steam.exe" -silent', location: 'Registry', scope: 'You', enabled: false }, { id: 'r1|SecurityHealth', name: 'SecurityHealth', command: '%windir%\\system32\\SecurityHealthSystray.exe', location: 'Registry', scope: 'All users', enabled: true }],
    dnsGet: () => '192.168.1.1',
    ping: () => [['oce', 'Oceania', 38], ['asia', 'Asia', 96], ['me', 'Middle East', 188], ['eu', 'Europe', 241], ['nae', 'NA East', 236], ['nac', 'NA Central', 214], ['naw', 'NA West', 168], ['br', 'Brazil', 0]].map(([id, name, avg]) => avg ? { id, name, avg, min: avg - 2, max: avg + 6, jitter: 1.4, loss: 0 } : { id, name, error: 'No reply (server may block ping)', loss: 100 }),
    log: () => ['16:20:01  AstroSpeed Tweaks 1.0.0 started (admin: true)', '16:20:44  Applying: Turn off Game DVR background recording', '16:20:44      ✓ HKCU\\System\\GameConfigStore → GameDVR_Enabled = 0'],
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
