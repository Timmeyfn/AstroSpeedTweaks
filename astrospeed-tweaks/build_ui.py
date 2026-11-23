#!/usr/bin/env python3
"""Inline fonts, logo, CSS and JS into one HTML file for the exe (and a preview build with a mock backend)."""
import base64, json, pathlib, re, subprocess, sys

root = pathlib.Path(__file__).parent
src = root / "ui" / "src"
b64 = lambda p: base64.b64encode(p.read_bytes()).decode()

tpl = (src / "index.html").read_text()
css = (src / "app.css").read_text()
js = (src / "app.js").read_text()

def build(prelude: str) -> str:
    out = tpl
    for k, v in {
        "{{FONT_MANROPE}}": b64(root / "ui/fonts/manrope.woff"),
        "{{LOGO}}": b64(root / "assets/logo.svg"),
        "{{CSS}}": css,
        "{{PRELUDE}}": prelude,
        "{{JS}}": js,
        "{{HILLS_DAY}}": b64(root / "assets/hills-day.webp"),
        "{{HILLS_NIGHT}}": b64(root / "assets/hills-night.webp"),
        "{{GAME_ART}}": json.dumps({f.stem: "data:image/webp;base64," + b64(f) for f in sorted((root / "assets/games").glob("*.webp"))}),
    }.items():
        out = out.replace(k, v)
    return out

dist = root / "ui" / "dist"
dist.mkdir(parents=True, exist_ok=True)
(dist / "index.html").write_text(build(""))
print("dist", (dist / "index.html").stat().st_size, "bytes")

if "--preview" in sys.argv:
    cat = json.loads((root / "ui/preview/catalog.json").read_text())
    # cleaner targets are defined in Go; extract titles for the preview from tools.go
    tools = (root / "tools.go").read_text()
    clean = [{"id": m[0], "title": m[1], "desc": m[2], "default": m[3] == "true"}
             for m in re.findall(r'\{ID: "([^"]+)", Title: "([^"]+)", Desc: "((?:[^"\\]|\\.)*)", Default: (true|false)', tools)]
    for c in clean:
        c["desc"] = c["desc"].replace('\\\\', '\\')
    prelude = "window.__CATALOG=" + json.dumps(cat) + ";window.__CLEAN=" + json.dumps(clean) + ";\n" + (src / "mock.js").read_text()
    (root / "ui/preview/index.html").write_text(build(prelude))
    print("preview written")
