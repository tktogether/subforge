# SubForge OpenWrt package definition

include $(TOPDIR)/rules.mk

PKG_NAME:=subforge
PKG_VERSION:=1.0.0
PKG_RELEASE:=1
PKG_LICENSE:=MIT
PKG_MAINTAINER:=SubForge

PKG_BUILD_PARALLEL:=1

include $(INCLUDE_DIR)/package.mk

# 这个包只打包预编译的静态二进制，SDK 端不重新编译 Go 源码。
# 因此必须显式声明 Build/Prepare：默认实现（include/package-defaults.mk 的
# Build/Prepare/Default）只有在 PKG_UNPACK 非空时才会定义，而本包没有 PKG_SOURCE，
# PKG_UNPACK 为空 —— 结果 $(PKG_BUILD_DIR) 是一个空目录，Package/subforge/install
# 里的 ./bin/subforge、./files/subforge.init 全部找不到。
# 同理默认的 Build/Compile 会在 $(PKG_BUILD_DIR) 里跑 `make`，那里没有 Makefile，
# 会报 "No targets specified and no makefile found"，所以必须覆盖成空实现。
define Build/Prepare
	@[ -f ./bin/subforge ] || { \
		echo "ERROR: deploy/openwrt/bin/subforge 不存在。"; \
		echo "这是打包用的预编译静态二进制，不属于源码树，必须先交叉编译："; \
		echo "  CGO_ENABLED=0 GOOS=linux GOARCH=<amd64|arm64> \\"; \
		echo "    go build -trimpath -mod=vendor -ldflags \"-s -w\" \\"; \
		echo "    -o deploy/openwrt/bin/subforge ./cmd/subforge"; \
		exit 1; \
	}
	rm -rf $(PKG_BUILD_DIR)
	mkdir -p $(PKG_BUILD_DIR)/bin $(PKG_BUILD_DIR)/files
	$(CP) ./bin/subforge $(PKG_BUILD_DIR)/bin/subforge
	$(CP) ./files/subforge.init $(PKG_BUILD_DIR)/files/subforge.init
	$(CP) ./files/subforge.conf $(PKG_BUILD_DIR)/files/subforge.conf
endef

define Build/Compile
	# 预编译二进制，无需编译步骤
endef

define Package/subforge
  SECTION:=net
  CATEGORY:=Network
  SUBMENU:=Web Servers/Proxies
  TITLE:=SubForge subscription converter
  DEPENDS:=+ca-bundle
endef

define Package/subforge/description
  Subscription converter with embedded mihomo parser. CGO-free single binary.
endef

define Package/subforge/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) ./bin/subforge $(1)/usr/bin/subforge
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/subforge.init $(1)/etc/init.d/subforge
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_DATA) ./files/subforge.conf $(1)/etc/config/subforge
endef

define Package/subforge/postinst
#!/bin/sh
[ -n "$$IPKG_INSTROOT" ] || {
	/etc/init.d/subforge enable
	/etc/init.d/subforge start
}
exit 0
endef

$(eval $(call BuildPackage,subforge))
