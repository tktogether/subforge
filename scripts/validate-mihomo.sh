#!/usr/bin/env bash
# 用 mihomo 实机校验 SubForge 生成的 clash.meta 产物是否真的能被内核加载。
#
# 用法:
#   SUBFORGE_MIHOMO=/path/to/mihomo(.exe) bash scripts/validate-mihomo.sh
#
# 为什么必须有这个脚本：`mihomo -t` 的默认行为是找工作目录下的 config.yaml，
# 找不到就**先生成一个初始模板，然后校验那个模板**——于是我们自己的坏配置也会
# 报 "test is successful"，等于白测。必须用绝对路径的 -f 指名配置文件，并核对
# 输出里确实出现 "test is successful"。
#
# 同样的两个陷阱已经固化成 go 测试 TestRender_MihomoValid（env 门控）。

set -euo pipefail

MIHOMO="${SUBFORGE_MIHOMO:-}"
if [[ -z "$MIHOMO" ]]; then
  echo "未设置 SUBFORGE_MIHOMO（指向 mihomo 二进制的路径）" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

cd "$ROOT"
go build -trimpath -o "$WORK/subforge" ./cmd/subforge

CFG="$WORK/mihomo.yaml"
"$WORK/subforge" convert \
  -file testdata/fixtures/mixed-subscription.txt \
  -name "subforge-validate" \
  -sort name \
  -out "$CFG"

# mktemp 在某些平台给相对路径，mihomo 的 -f 必须能吃绝对路径。
CFG="$(cd "$(dirname "$CFG")" && pwd)/$(basename "$CFG")"

echo "--- mihomo -t ---"
set +e
"$MIHOMO" -t -f "$CFG" 2>&1 | tee "$WORK/mihomo.log"
rc=${PIPESTATUS[0]}
set -e

if [[ "$rc" -ne 0 ]]; then
  echo "mihomo -t 返回 $rc：产物不可加载" >&2
  exit 1
fi
if ! grep -q "test is successful" "$WORK/mihomo.log"; then
  echo "mihomo 输出中没有 'test is successful'（可能校验的是它自己生成的初始模板）" >&2
  exit 1
fi

echo "OK: 产物可被 mihomo 加载"
