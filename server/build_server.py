#!/usr/bin/env python3
"""Builds server/astrospeed-server.js (the ONE file you paste into Cloudflare) from server/src/*.

Each page is embedded as a JSON string, so nothing inside a page (backslashes, backticks,
"${", line breaks) can ever break the server file.
"""
import json, pathlib, sys

here = pathlib.Path(__file__).resolve().parent
src = here / "src"
js = (src / "server.js").read_text(encoding="utf-8")
pages = {"__SITE_HTML__": "site.html", "__ADMIN_HTML__": "admin.html", "__THANKS_HTML__": "thanks.html", "__SIMPLE_HTML__": "simple.html"}
for marker, name in pages.items():
    html = (src / name).read_text(encoding="utf-8")
    if marker not in js:
        sys.exit(f"marker {marker} missing from server.js")
    # json.dumps gives a valid JS string literal; also stop "</script>" ending an inline script early
    js = js.replace(marker, json.dumps(html, ensure_ascii=False).replace("</", "<\\/"))
out = here / "astrospeed-server.js"
out.write_text(js, encoding="utf-8")
print(f"wrote {out.name}: {len(js):,} bytes")
# key hoster build: same code, only answers key routes; no website, shop or downloads
keys = js.replace('const ROLE = "all";', 'const ROLE = "keys";', 1).replace("AstroSpeed server — ONE file for everything.", "AstroSpeed KEY HOSTER (rapid-forest) — keys only.", 1)
assert 'const ROLE = "keys";' in keys
(here / "key-hoster.js").write_text(keys, encoding="utf-8")
print("wrote key-hoster.js")
