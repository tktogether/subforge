# OpenWrt 打包验证报告

本报告记录 `deploy/openwrt/` 打包链路在官方 SDK 上的验证过程与产物数据。
操作步骤见 [deploy/README.md](../../deploy/README.md)，本报告只记录验证结论与原始数据。

| 项目 | 内容 |
| --- | --- |
| 验证对象 | `deploy/openwrt/`（OpenWrt 包配方与 init 脚本） |
| 验证日期 | 2026-10-20 |
| 验证环境 | WSL2 Ubuntu 22.04，OpenWrt SDK 24.10.4（x86_64 主机） |
| 验证方式 | 用官方 SDK 执行 `make package/subforge/compile V=s`，再解开 ipk 检查其结构与内容 |
| 结论 | **可以打包**，但仓库原有配方缺少 `Build/Prepare`，直接编译会失败；修复后两个目标架构均打包成功 |

---

## 1. 原有配方为何失败

仓库最初的 `deploy/openwrt/Makefile` 没有声明 `PKG_SOURCE`，也没有自定义 `Build/Prepare`。
OpenWrt 构建系统的默认 `Build/Prepare/Default` 只在 `PKG_UNPACK` 非空时才定义，
因此 `$(PKG_BUILD_DIR)` 是一个空目录，`Package/subforge/install` 中引用的 `./bin/subforge`
与 `./files/*` 全部找不到，编译以退出码 2 结束：

```
make[3]: *** No targets specified and no makefile found.  Stop.
```

修复方式（已提交，见 commit `804720b`）：

1. 删除无效的 `PKG_BUILD_DEPENDS:=golang/host`——本包不编译 Go 代码，二进制由外部交叉编译后放入。
2. 新增 `Build/Prepare`：先校验 `./bin/subforge` 存在（不存在则报错并打印交叉编译命令），
   再把二进制与 `files/` 下的两个文件复制进 `$(PKG_BUILD_DIR)`。
3. 新增空的 `Build/Compile`：预编译二进制不需要编译步骤。

---

## 2. 产物

| 产物 | 目标架构 | 字节 | SHA-256 |
| --- | --- | --- | --- |
| `subforge_1.0.0-r1_x86_64.ipk` | `x86_64` | 15,451,470 | `5ba5d0f87946a21c666678fea50207a0f8be54f07656c9d13b96463bd9488fce` |
| `subforge_1.0.0-r1_aarch64_cortex-a72.ipk` | `aarch64_cortex-a72` | 14,264,239 | `78050022c2b0c8ca191174468eb613ae8834625233260e7e906381f2392844cc` |

产物落在 SDK 的 `bin/packages/<PKGARCH>/base/` 目录下。

包内二进制（x86_64 包）：

- 大小 40,068,162 字节，SHA-256 `a9fbb4f99e77345c491cfad948648c5dd76b932a67302f849640a1faf9814585`
- `ELF 64-bit LSB executable, x86-64, statically linked`
- `version` 输出 `subforge dev (commit none, built unknown)`——因为放入 `deploy/openwrt/bin/`
  的二进制由 `make cross` 产出，未注入版本信息

---

## 3. ipk 的结构

`.ipk` 是 **gzip 压缩的 tar**（不是 `ar` 归档，`ar t` 无法识别），解开后包含三个成员：

| 成员 | 说明 |
| --- | --- |
| `debian-binary` | 内容为版本号 `2.0` |
| `control.tar.gz` | 元数据：`control`、`postinst`、`postinst-pkg`、`prerm` |
| `data.tar.gz` | 实际安装内容 |

`control` 的关键字段：

```
Package: subforge
Version: 1.0.0-r1
Depends: libc, ca-bundle
Architecture: x86_64
Installed-Size: 40079360
License: MIT
Section: net
```

`data.tar.gz` 内恰好三个文件：

| 路径 | 权限 | 说明 |
| --- | --- | --- |
| `usr/bin/subforge` | 755 | 主程序 |
| `etc/init.d/subforge` | 755 | procd init 脚本 |
| `etc/config/subforge` | 644 | UCI 配置文件 |

`postinst` 先加载 `/lib/functions.sh` 并调用 `default_postinst`；`postinst-pkg` 执行
`/etc/init.d/subforge enable` 与 `start`。

---

## 4. 已知问题与未验证项

1. **UCI 配置文件当前无人读取。** `deploy/openwrt/files/subforge.init` 没有调用
   `config_load` / `config_get`，`deploy/openwrt/files/subforge.conf` 中的选项不会生效；
   脚本使用固定的环境变量启动服务。这一点属于已确认的缺陷，尚未修复。
2. **未在真实设备上安装过。** 未在真实 OpenWrt 设备或 rootfs 上执行 `opkg install`，
   因此 `postinst` 的实际行为、`rc.d` 软链接的建立、procd 拉起服务、`ca-bundle`
   依赖解析、`opkg` 对 gzip 外层格式的接受度均未验证。
3. **`Installed-Size` 与文件之和存在差值。** SDK 在 `control` 中记录的 `Installed-Size` 为
   40,079,360 字节，而包内三个文件之和为 40,068,745 字节（40,068,162 + 376 + 207），
   差值 10,615 字节应为打包工具计入的 tar 元数据与对齐开销，未逐字节核对。
   该字段只影响包管理器的展示与空间预检，不影响安装。

---

## 5. 复现方式

```bash
# 前置：先产出目标架构的二进制
make cross
cp bin/subforge-linux-amd64 deploy/openwrt/bin/subforge

# 在官方 SDK 根目录下执行
cp -r <repo>/deploy/openwrt package/subforge
make package/subforge/compile V=s

# 检查产物
tar -xzf bin/packages/x86_64/base/subforge_1.0.0-r1_x86_64.ipk -C /tmp/ipk
```

---

## 相关文档

- [部署与打包](../../deploy/README.md)（仓库 `deploy/README.md`）
- [安装与部署](../wiki/安装与部署.md)（Wiki）
- [v0.1.2 发布验证报告](v0.1.2-release-verification.md)
