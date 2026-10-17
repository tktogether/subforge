# Aethersailor 模板夹具来源

本目录存放**逐字节取自上游**的 Aethersailor（Custom_OpenClash_Rules）订阅转换模板，
仅用于兼容性核查与回归测试；不进入发行物。

## 上游信息

| 项 | 值 |
|---|---|
| 仓库 | https://github.com/Aethersailor/Custom_OpenClash_Rules |
| 模板目录 | https://github.com/Aethersailor/Custom_OpenClash_Rules/tree/main/cfg |
| 抓取时的 main HEAD | `f90155ba7d012bbaad1b91d4abde0900194b157d`（2026-10-17T08:29:45Z，`chore(rules): auto generate derived rule files`） |
| 抓取日期 | 2026-10-17（Asia/Shanghai） |
| 许可 | CC-BY-SA-4.0（模板文件头部保留原作者与项目地址注释） |

## 模板清单（8 个正式模板 + 1 个兼容文件）

上游 `cfg/README.md` 明确定义「4 个普通版 + 4 个故障转移版 = 8 个模板」，
另有一个由生成流程自动同步的兼容文件 `Custom_Clash_Mainland.ini`。

| 文件 | 上游 blob SHA-1 | 字节 | sha256（前 16） | 说明 |
|---|---|---|---|---|
| `Custom_Clash.ini` | `6a7496233ff35209832e88e37e96741d3c008a2a` | 12707 | `1bb4f64365a4a0f3` | 标准版 |
| `Custom_Clash_Fallback.ini` | `eeb10e5bbef2e5d186271004b4cd3df69626508c` | 37611 | `b2c757347792c46c` | 标准故障转移版 |
| `Custom_Clash_Lite.ini` | `17ce62118a9520bccf73f97f93c99dc7cb1eacf4` | 7832 | `23753309f3a5f473` | 轻量版 |
| `Custom_Clash_Lite_Fallback.ini` | `adb67e3fc13fa2587c2b71a2dbe9ce26e93aaa37` | 16904 | `6ffce47f708e0107` | 轻量故障转移版 |
| `Custom_Clash_GFW.ini` | `f138a918de7278532e5975b7330ffe6fc20cb256` | 1269 | `e6327808584da128` | 极简 GFW 版 |
| `Custom_Clash_GFW_Fallback.ini` | `955e7bb0c42f58296f30663dbbbdf854dada77ae` | 1201 | `fe96bcd1e468ca4a` | 极简 GFW 故障转移版 |
| `Custom_Clash_Full.ini` | `315c2ca2ebc25a28e614a014cf714d110b958711` | 30324 | `aeea0542a03d31c5` | 重度分流版 |
| `Custom_Clash_Full_Fallback.ini` | `84eaf62e1864dbeef9a3da72399f59142d71855d` | 29459 | `7f725715f2726dc9` | 重度分流故障转移版 |
| `Custom_Clash_Mainland.ini` | `6a7496233ff35209832e88e37e96741d3c008a2a` | 12707 | `1bb4f64365a4a0f3` | 兼容文件，与 `Custom_Clash.ini` **逐字节相同** |

下载地址形如
`https://raw.githubusercontent.com/Aethersailor/Custom_OpenClash_Rules/main/cfg/<文件名>`。

**完整性校验**：本地文件的 git blob SHA-1 与 GitHub Contents API 返回的 `sha` 全部一致，
即夹具与上游逐字节相同（LF 行尾、UTF-8、无 BOM）。

## 合成探针（非上游内容）

以下文件是核查过程中自建的最小复现输入，**不是**上游模板，只服务于本目录的兼容性回归：

| 文件 | 用途 |
|---|---|
| `_probe-http-member.ini` | 成员段以 `http` 开头时的解析行为（见报告 P1） |
| `_probe-empty-group.ini` | 零成员分组的渲染行为（见报告 P2） |
| `_probe-empty-group2.ini` | 合法分组与零成员分组并列时的行为（见报告 P2） |
| `_probe-sub-httpnode.b64` | 仅含一个名为 `http-node` 的 vmess 节点的 base64 订阅，用于区分「`http` 前缀段是测活地址还是成员正则」 |

核查报告：`docs/m2-aethersailor-compat.md`。
