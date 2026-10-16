#!/usr/bin/env python3
"""同输入对拍 runner（修正版）：subconverter 以 HTTP 服务方式在 25501 端口对拍。

四个参考项目基线：
- subconverter / SCE：tools/scv/goldens（subconverter v0.9.0 实机产物）
- Aethersailor：Custom_Clash_Full.ini 模板（tools/local-run）对拍记录
- Sub-Store：filter→operate 语义层（internal/transform）
- sub-poly：分享链接编码语义层（internal/render/sharelink）
"""
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
import base64

SCV = "http://127.0.0.1:25501"
MIHOMO = r"e:\Projects\subforge\.compat\bin\mihomo-windows-amd64-compatible.exe"
BASE = "http://127.0.0.1:25500"
SUB = "http://127.0.0.1:8899/sub-b64.txt"
OUTDIR = r"e:\Projects\subforge\.compat\out"
os.makedirs(OUTDIR, exist_ok=True)

results = []


def case(name, ok, detail=""):
    results.append((name, ok, detail))
    print(f"[{'PASS' if ok else 'FAIL'}] {name}  {detail[:140]}")


def sf_get(target, extra=None):
    params = {"target": target, "url": SUB}
    if extra:
        params.update(extra)
    q = urllib.parse.urlencode(params)
    try:
        with urllib.request.urlopen(f"{BASE}/sub?{q}", timeout=60) as r:
            return r.status, dict(r.headers), r.read()
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()


def scv_get(target):
    q = urllib.parse.urlencode({"target": target, "url": SUB})
    try:
        with urllib.request.urlopen(f"{SCV}/sub?{q}", timeout=60) as r:
            return r.status, dict(r.headers), r.read()
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()


def decode_mixed(body):
    text = body.decode("utf-8", "replace").strip()
    try:
        return base64.b64decode(text + "===").decode("utf-8", "replace").splitlines()
    except Exception:
        return text.splitlines()


def b64decode_safe(s):
    """解码 base64，兼容标准与 URL-safe 字母表。"""
    try:
        return base64.urlsafe_b64decode(s + "===")
    except Exception:
        try:
            return base64.b64decode(s + "===")
        except Exception:
            return None


def extract_node_name(line):
    """从分享链接中提取节点名，支持有/无 # 后缀的链接。"""
    m = re.search(r"#([^#]+)$", line)
    if m:
        return m.group(1).strip()
    if line.startswith("ssr://"):
        data = b64decode_safe(line[6:])
        if data:
            text = data.decode("utf-8", "replace")
            if "/?" in text:
                for kv in text.split("/?", 1)[1].split("&"):
                    if kv.startswith("remarks="):
                        remarks = b64decode_safe(kv[8:])
                        if remarks:
                            return remarks.decode("utf-8", "replace").strip()
    if line.startswith("vmess://"):
        data = b64decode_safe(line[8:])
        if data:
            try:
                j = json.loads(data.decode("utf-8", "replace"))
                if "ps" in j and j["ps"]:
                    return j["ps"].strip()
            except Exception:
                pass
    return None


def sf_nodes(body, target):
    text = body.decode("utf-8", "replace")
    if target in ("mixed", "ss", "ssd", "ssr", "trojan", "v2ray"):
        text = base64.b64decode(text.strip() + "===").decode("utf-8", "replace")
    names = []
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        name = extract_node_name(line)
        if name:
            names.append(name)
    return names


print("== A. subconverter 同输入对拍（12 节点 fixture）==")
for target in ["mixed", "ss", "ssr", "trojan", "v2ray"]:
    st, h, body = sf_get(target)
    st2, h2, body2 = scv_get(target)
    if st2 != 200:
        case(f"对拍 {target}", False, f"scv HTTP {st2}")
        continue
    sf = sf_nodes(body, target)
    scv = sf_nodes(body2, target)
    ok = set(scv) <= set(sf)
    case(f"对拍 {target}（scv 节点集 ⊆ sf）", ok,
         f"sf={len(sf)} scv={len(scv)}  scv-only={sorted(set(scv)-set(sf))[:3]}")

print("== B. Aethersailor Custom_Clash_Full.ini 语义对拍 ==")
tpl = r"e:\Projects\subforge\tools\local-run\Custom_Clash_Full.ini"
tpl_url = "http://127.0.0.1:8899/Custom_Clash_Full.ini"
# 先把模板放进 upstream 目录
import shutil
shutil.copy(tpl, r"e:\Projects\subforge\.compat\upstream\Custom_Clash_Full.ini")
st, h, body = sf_get("clash.meta", {"config": tpl_url})
text = body.decode("utf-8", "replace")
groups = re.findall(r"-\s*name:\s*['\"]?([^,'\n\"{}]+)", text)
rules = re.findall(r"^\s*-\s*[\"']?(GEOSITE|IP-CIDR|GEOIP|DST-PORT|DOMAIN|DOMAIN-SUFFIX|DOMAIN-KEYWORD|SRC-PORT|MATCH)", text, re.M)
case("Aethersailor 模板：分组数>50", len(groups) > 50, f"groups={len(groups)}")
case("Aethersailor 模板：规则数>80", len(rules) > 80, f"rules={len(rules)}")
open(os.path.join(OUTDIR, "aethersailor.yaml"), "wb").write(body)

print("== C. mihomo -t 实机加载（Aethersailor 产物）==")
ay = os.path.join(OUTDIR, "aethersailor.yaml")
mihomo_home = os.path.join(OUTDIR, "mihomo-home")
os.makedirs(mihomo_home, exist_ok=True)
mihomo_env = dict(os.environ)
mihomo_env["MIHOMO_CONFIG_DIR"] = mihomo_home
mihomo_env["XDG_CONFIG_HOME"] = mihomo_home
try:
    p = subprocess.run([MIHOMO, "-t", "-d", mihomo_home, "-f", ay], capture_output=True, timeout=60, env=mihomo_env)
    case("mihomo -t 加载 Aethersailor 产物", p.returncode == 0,
         (p.stdout + p.stderr).decode("utf-8", "replace")[:120])
except Exception as e:
    case("mihomo -t 加载", False, str(e)[:100])

print("== D. clash.meta 产物 mihomo -t 实机加载 ==")
st, h, body = sf_get("clash.meta")
open(os.path.join(OUTDIR, "clashmeta.yaml"), "wb").write(body)
try:
    p = subprocess.run([MIHOMO, "-t", "-d", mihomo_home, "-f", os.path.join(OUTDIR, "clashmeta.yaml")],
                       capture_output=True, timeout=60, env=mihomo_env)
    case("mihomo -t 加载 clash.meta 产物", p.returncode == 0,
         (p.stdout + p.stderr).decode("utf-8", "replace")[:120])
except Exception as e:
    case("mihomo -t 加载 clash.meta", False, str(e)[:100])

print()
fails = [r for r in results if not r[1]]
print(f"TOTAL {len(results)}  PASS {len(results)-len(fails)}  FAIL {len(fails)}")
for name, ok, detail in fails:
    print(f"  FAIL: {name}  {detail[:200]}")
sys.exit(1 if fails else 0)
