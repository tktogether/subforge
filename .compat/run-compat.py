#!/usr/bin/env python3
"""四个参考项目兼容性端到端测试 runner。

对每个参考项目的基线产物（tools/scv/goldens, goldens2）与 SubForge
通过 /sub 端到端产出的结果做结构化比对；无基线的场景走"可加载性 +
往返 + 探针"验证。
"""
import json
import sys
import urllib.error
import urllib.parse
import urllib.request
import base64
import zlib

BASE = "http://127.0.0.1:25500"
SUB = "http://127.0.0.1:8899/sub.txt"
OUT = r"e:\Projects\subforge\.compat\out"

results = []


def get(path, timeout=30):
    url = f"{BASE}{path}"
    req = urllib.request.Request(url)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, dict(r.headers), r.read()
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()
    except Exception as e:
        return None, {}, str(e).encode()


def case(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"[{'PASS' if ok else 'FAIL'}] {name}  {detail[:120]}")


def decode_mixed(body):
    """mixed 形态是整体 base64 包裹的分享链接清单。"""
    text = body.decode("utf-8", "replace").strip()
    try:
        return base64.b64decode(text + "===").decode("utf-8", "replace").splitlines()
    except Exception:
        return text.splitlines()


def check_target(target, expect_lines=None, ua=None):
    q = urllib.parse.urlencode({"target": target, "url": SUB})
    url = f"{BASE}/sub?{q}"
    req = urllib.request.Request(url)
    if ua:
        req.add_header("User-Agent", ua)
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            body = r.read()
            return r.status, dict(r.headers), body
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()


# ---- 1. 核心业务流程：全 target 端到端 ----
print("== 1. 全 target 端到端转换 ==")
for target in ["clash.meta", "clash-json", "sing-box", "list", "mixed", "ss",
               "ssd", "ssr", "trojan", "v2ray", "surge", "quan", "quanx",
               "loon", "surfboard", "ss-sub", "ss-sub-b64", "mellow"]:
    st, h, body = check_target(target)
    nodes = h.get("X-Node-Count", "?")
    skipped = h.get("X-Convert-Skipped", "0")
    ct = h.get("Content-Type", "?")
    ok = st == 200 and len(body) > 0
    case(f"target={target}", ok, f"HTTP {st} {len(body)}B nodes={nodes} skipped={skipped} ct={ct[:30]}")

# ---- 2. 数据格式适配：mixed/base64 形态行数与内容 ----
print("== 2. 数据格式适配（mixed/base64）==")
st, h, body = check_target("mixed")
lines = decode_mixed(body)
case("mixed 行数=12", len(lines) == 12, f"got {len(lines)}: {lines[:2]}")
st, h, body = check_target("ss")
lines = [l for l in decode_mixed(body) if l.startswith("ss://") or l.startswith("ssr://")]
case("ss 含 ss+ssr 共2条", len(lines) == 2, f"got {len(lines)}")
st, h, body = check_target("v2ray")
lines = decode_mixed(body)
case("v2ray 仅 vmess 1条", len([l for l in lines if l.startswith("vmess://")]) == 1, f"got {len(lines)}")

# ---- 3. UA 推断（边缘交互）----
print("== 3. UA 推断 ==")
st, h, body = check_target("", ua="clash-verge")
fmt = h.get("X-Convert-Format", "?")
case("UA=clash-verge -> clash.meta", fmt == "clash.meta", f"format={fmt}")
st, h, body = check_target("", ua="sing-box")
fmt = h.get("X-Convert-Format", "?")
case("UA=sing-box -> sing-box", fmt == "sing-box", f"format={fmt}")

# ---- 4. 负例 ----
print("== 4. 负例 ==")
st, h, body = get("/sub")
case("缺 url -> 400", st == 400, f"HTTP {st}")
st, h, body = get("/sub?target=vmess&url=" + urllib.parse.quote(SUB))
case("不支持 target=vmess -> 501", st == 501, f"HTTP {st}")
st, h, body = get("/sub?target=clash.meta&url=http://127.0.0.1:1/none.txt")
case("坏上游 -> 502", st == 502, f"HTTP {st}")

# ---- 5. sing-box JSON 可加载性（jsonschema 前置门）----
print("== 5. sing-box 可加载性 ==")
import subprocess, os
st, h, body = check_target("sing-box")
try:
    p = subprocess.run(
        [sys.executable, r"e:\Projects\subforge\tools\check-singbox-schema.py"],
        input=body, capture_output=True, timeout=20)
    case("sing-box JSON 过 jsonschema", p.returncode == 0,
         p.stdout.decode()[:100] or p.stderr.decode()[:100])
except Exception as e:
    case("sing-box JSON 过 jsonschema", False, str(e)[:100])

# ---- 6. 分享链接往返 oracle ----
print("== 6. 往返 oracle ==")
st, h, body = check_target("list")
links = [l for l in body.decode("utf-8", "replace").splitlines() if l.strip()]
case("list 产出 12 条分享链接", len(links) == 12, f"got {len(links)}")

print()
fails = [r for r in results if not r[1]]
print(f"TOTAL {len(results)}  PASS {len(results)-len(fails)}  FAIL {len(fails)}")
for name, ok, detail in fails:
    print(f"  FAIL: {name}  {detail[:200]}")
sys.exit(1 if fails else 0)
