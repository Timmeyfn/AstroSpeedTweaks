#!/usr/bin/env python3
"""Turns every .bat file in the "custom" folder into a switch in AstroSpeed.

Each .bat becomes one tweak in a "Your tweaks" section. Lines the app understands
(reg add / reg delete / sc config / schtasks / bcdedit) get an automatic backup and
undo. Other plain commands are run as-is (no automatic undo). Lines that only make
sense inside a .bat (if, for, goto, variables, pause, ...) are skipped and listed
in the build report.

Usage: python tools/bat2tweak.py custom
Writes custom_tweaks.go next to catalog.go.
"""
import json
import pathlib
import re
import shlex
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
OUT = ROOT / "custom_tweaks.go"
NO_UNDO = "rem AstroSpeed: this line has no automatic undo"

q = json.dumps  # a JSON string is also a valid Go string literal

SILENT_WORDS = {  # harmless .bat housekeeping, ignored without a mention
    "echo", "cls", "title", "color", "setlocal", "endlocal", "chcp", "mode", "exit",
    "pause", "prompt", "cd", "pushd", "popd", "@echo",
}
SKIP_WORDS = {"goto", "call", "set", "timeout", "choice", "start", "shutdown", "if", "for", "else", "ping"}
SKIP_REASONS = {
    "shutdown": "would restart or shut down the PC in the middle of Optimize",
    "start": "opens programs or windows",
    "pause": "waits for a key press",
    "timeout": "only waits",
    "choice": "asks the user a question",
    "ping": "usually only used as a wait",
}
ROOTS = {"HKLM": "HKLM", "HKEY_LOCAL_MACHINE": "HKLM", "HKCU": "HKCU", "HKEY_CURRENT_USER": "HKCU"}
SC_START = {"disabled": "disabled", "demand": "manual", "auto": "auto", "delayed-auto": "auto"}


def read_lines(path):
    raw = path.read_bytes()
    for enc in ("utf-8-sig", "cp1252", "latin-1"):
        try:
            text = raw.decode(enc)
            break
        except UnicodeDecodeError:
            continue
    lines, buf, start = [], "", None
    for n, ln in enumerate(text.splitlines(), 1):
        s = ln.rstrip()
        if start is None:
            start = n
        if s.endswith("^"):  # line continuation
            buf += s[:-1]
            continue
        lines.append((start, buf + s))
        buf, start = "", None
    if buf:
        lines.append((start, buf))
    return lines


def split_args(line):
    try:
        return shlex.split(line, posix=False)
    except ValueError:
        return line.split()


def unq(s):
    return s[1:-1] if len(s) >= 2 and s[0] == s[-1] == '"' else s


def flag(args, name):
    """value after /name (case-insensitive), or None"""
    for i, a in enumerate(args):
        if a.lower() == name and i + 1 < len(args):
            return unq(args[i + 1])
    return None


def split_key(key):
    key = unq(key).strip("\\")
    head, _, rest = key.partition("\\")
    root = ROOTS.get(head.upper())
    return root, rest


def convert_reg(args):
    verb = args[1].lower() if len(args) > 1 else ""
    if len(args) < 3:
        return None
    root, path = split_key(args[2])
    if not root or not path:
        return None
    lower = [a.lower() for a in args]
    if "/ve" in lower:
        return None  # default value: leave to the command fallback
    name = flag(args, "/v")
    if name is None:
        return None
    if verb == "delete":
        return f"RegDel({q(root)}, {q(path)}, {q(name)})"
    if verb != "add":
        return None
    typ = (flag(args, "/t") or "REG_SZ").upper()
    data = flag(args, "/d")
    if data is None:
        data = ""
    if typ in ("REG_DWORD", "REG_QWORD"):
        try:
            num = int(data, 0) if data.lower().startswith("0x") else int(data)
        except ValueError:
            return None
        if num < 0:
            return None
        fn = "RegD" if typ == "REG_DWORD" else "RegQ"
        if typ == "REG_DWORD" and num > 0xFFFFFFFF:
            return None
        return f"{fn}({q(root)}, {q(path)}, {q(name)}, {num})"
    if typ == "REG_SZ":
        return f"RegS({q(root)}, {q(path)}, {q(name)}, {q(data)})"
    return None  # binary / multi-string / expand-string: command fallback


def convert_sc(args):
    if len(args) >= 4 and args[1].lower() == "config":
        svc = unq(args[2])
        for i, a in enumerate(args):
            if a.lower() == "start=" and i + 1 < len(args):
                st = SC_START.get(args[i + 1].lower())
                if st:
                    return f"Svc({q(svc)}, {q(st)})"
            if a.lower().startswith("start=") and len(a) > 6:
                st = SC_START.get(a[6:].lower())
                if st:
                    return f"Svc({q(svc)}, {q(st)})"
    return None


def convert_schtasks(args):
    lower = [a.lower() for a in args]
    if "/change" in lower and "/disable" in lower:
        tn = flag(args, "/tn")
        if tn:
            return f"Task({q(tn)})"
    return None


def convert_bcdedit(args):
    lower = [a.lower() for a in args]
    rest = [a for a in args[1:] if a.lower() not in ("{current}",)]
    if lower[1:2] == ["/set"] and len(rest) == 3:
        return f"BCD({q(rest[1])}, {q(rest[2])})"
    if lower[1:2] == ["/deletevalue"] and len(rest) == 2:
        return f"BCD({q(rest[1])}, \"\")"
    return None


KNOWN_UNDO = [  # commands with a well-known opposite
    (re.compile(r"^powercfg\s+[-/]h(ibernate)?\s+off$", re.I), "powercfg /hibernate on"),
    (re.compile(r"^netsh\s+int(erface)?\s+teredo\s+set\s+state\s+disabled$", re.I), "netsh interface teredo set state default"),
]


def convert_line(line):
    """returns (kind, go_code_or_reason)"""
    s = line.strip()
    if s.startswith("@"):
        s = s[1:].strip()
    if not s or s.startswith("::") or s.lower() == "rem" or s.lower().startswith("rem "):
        return "blank", None
    if s.startswith(":") or s in ("(", ")"):
        return "blank", None
    s = re.sub(r"\s*(>>?|2>&1|2>)\s*nul\b", "", s, flags=re.I).strip()  # drop output redirection
    s = re.sub(r"\s*2>&1\s*$", "", s).strip()
    if not s:
        return "blank", None
    word = s.split()[0].lower()
    if word in SILENT_WORDS:
        return "blank", None
    if word in SKIP_WORDS:
        return "skip", SKIP_REASONS.get(word, f"'{word}' only works inside a .bat file")
    if word == "net" and len(s.split()) > 1 and s.split()[1].lower() == "session":
        return "blank", None  # admin check; AstroSpeed already runs as admin
    if re.search(r"%%|%~|%[0-9*]|![A-Za-z_]+!|%errorlevel%", s, re.I):
        return "skip", "uses .bat-only variables"
    if "&&" in s or "||" in s or re.search(r"(^|\s)\|\s", s):
        return "skip", "joins several commands together; put each on its own line"
    args = split_args(s)
    conv = None
    if word in ("reg", "reg.exe"):
        conv = convert_reg(args)
    elif word in ("sc", "sc.exe"):
        conv = convert_sc(args)
    elif word in ("schtasks", "schtasks.exe"):
        conv = convert_schtasks(args)
    elif word in ("bcdedit", "bcdedit.exe"):
        conv = convert_bcdedit(args)
    if conv:
        return "undo", conv
    for rx, rev in KNOWN_UNDO:
        if rx.match(s):
            return "undo", f"Cmd({q(s)}, {q(rev)})"
    return "cmd", f"Cmd({q(s)}, {q(NO_UNDO)})"


def nice_title(stem):
    t = re.sub(r"[_\-]+", " ", stem).strip()
    return t[:1].upper() + t[1:] if t else "Custom tweaks"


def main():
    folder = (ROOT / (sys.argv[1] if len(sys.argv) > 1 else "custom")).resolve()
    bats = sorted(p for p in folder.glob("*.bat")) + sorted(p for p in folder.glob("*.cmd"))
    tweaks, report, ids = [], [], set()
    for bat in bats:
        tid = "bat-" + (re.sub(r"[^a-z0-9]+", "-", bat.stem.lower()).strip("-") or "custom")
        while tid in ids:
            tid += "-2"
        ids.add(tid)
        desc = ""
        actions, n_undo, n_cmd, skipped = [], 0, 0, []
        depth, menu = 0, False
        for n, line in read_lines(bat):
            st = line.strip().lstrip("@")
            low = st.lower()
            if re.match(r"^(goto|choice)\b", low) or re.match(r"^set\s+/p\b", low):
                menu = True
            opens = st.endswith("(")
            closes = st.startswith(")")
            if depth > 0 or (opens and re.match(r"^(if|for)\b", low)):
                if closes:
                    depth -= 1
                if opens:
                    depth += 1
                first = (low.split() or [""])[0]
                if st not in (")", "(") and not re.match(r"^\)\s*else\s*\($", st) and first not in SILENT_WORDS:
                    skipped.append((n, st, "inside an if/for block, so it only runs sometimes"))
                continue
            if not desc and (st.lower().startswith("rem ") or st.startswith("::")):
                d = st[3:].strip(" :") if st.startswith("::") else st[4:].strip()
                if len(d) > 8 and not set(d) <= set("=-*#~ "):
                    desc = d
            kind, val = convert_line(line)
            if kind == "undo":
                actions.append(val); n_undo += 1
            elif kind == "cmd":
                actions.append(val); n_cmd += 1
            elif kind == "skip":
                skipped.append((n, line.strip(), val))
        report.append((bat.name, n_undo, n_cmd, skipped, menu))
        if not actions:
            continue
        note = f"Imported from {bat.name}."
        if n_cmd:
            note += f" {n_cmd} of its {n_undo + n_cmd} commands can't be undone automatically."
        tweaks.append(f"""	{{
		ID: {q(tid)}, Cat: "custom", Group: {q("From " + bat.name)}, Title: {q(nice_title(bat.stem))},
		Desc:    {q(desc or f"Runs the commands from {bat.name}.")},
		Benefit: {q("Your own tweaks, applied with the rest of Optimize.")},
		Note:    {q(note)},
		Impact:  2, Risk: "moderate", Profile: 2,
		Actions: []Action{{
			{(","+chr(10)+chr(9)*3).join(actions)},
		}},
	}},""")

    code = "// Code generated by tools/bat2tweak.py from the custom folder. DO NOT EDIT; edit the .bat files and rebuild.\n\npackage main\n"
    if tweaks:
        code += """
func init() {
	Categories = append(Categories, Category{ID: "custom", Title: "Your tweaks", Subtitle: "Tweaks imported from the .bat files in the custom folder.", Icon: "spark"})
	Catalog = append(Catalog, []Tweak{
""" + "\n".join(tweaks) + """
	}...)
}
"""
    OUT.write_text(code, encoding="utf-8")

    # ── build report ──
    print()
    if not bats:
        print(f"No .bat files in {folder}. Building without custom tweaks.")
    for name, nu, nc, skipped, menu in report:
        print(f"{name}: {nu} with automatic undo, {nc} run as commands (no undo), {len(skipped)} skipped")
        if menu:
            print("   WARNING: this .bat has a menu (goto/choice). Every option's commands are applied,")
            print("   one after another. Split it into one .bat per option so each gets its own switch.")
        for n, text, why in skipped:
            short = text if len(text) <= 70 else text[:67] + "..."
            print(f"   line {n}: {short}")
            print(f"      skipped: {why}")
    print()


if __name__ == "__main__":
    main()
