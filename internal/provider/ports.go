package provider

import "github.com/blang/semver"

// This is for maintaining backwards compatibility in v0.4.x for dokku versions
// < 0.32, will probably deprecate < 0.32 support from v0.5.0

func shouldUseProxyPortsCmd() bool {
	proxyDeprecatedAt := "< 0.32.0"
	compat, _ := semver.ParseRange(proxyDeprecatedAt)

	return compat(DOKKU_VERSION)
}

func portAddCmd() string {
	if shouldUseProxyPortsCmd() {
		return "proxy:ports-add"
	}
	return "ports:add"
}

func portRemoveCmd() string {
	if shouldUseProxyPortsCmd() {
		return "proxy:ports-remove"
	}
	return "ports:remove"
}

func portReadCmd() string {
	if shouldUseProxyPortsCmd() {
		return "proxy:ports"
	}
	return "ports:list"
}