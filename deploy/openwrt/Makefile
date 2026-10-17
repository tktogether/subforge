# SubForge OpenWrt package definition

include $(TOPDIR)/rules.mk

PKG_NAME:=subforge
PKG_VERSION:=1.0.0
PKG_RELEASE:=1
PKG_LICENSE:=MIT
PKG_MAINTAINER:=SubForge

PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1

include $(INCLUDE_DIR)/package.mk

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
