package provider

import (
	"strings"

	"github.com/blang/semver"
)

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

func portSetCmd() string {
	if shouldUseProxyPortsCmd() {
		return "proxy:ports-set"
	}
	return "ports:set"
}

// portHostKey returns the "scheme:hostPort" part of a
// "scheme:hostPort:containerPort" mapping. Dokku can only route one container
// port per scheme and host port, so two mappings with the same key conflict.
func portHostKey(mapping string) string {
	parts := strings.Split(strings.TrimSpace(mapping), ":")
	if len(parts) < 2 {
		return strings.TrimSpace(mapping)
	}
	return parts[0] + ":" + parts[1]
}

// mergePorts returns the full set of port mappings an app should have after
// adding toAdd to existing. Existing mappings on the same scheme:hostPort as
// one being added are dropped, so the new mapping replaces them (e.g
// http:80:8080 replaces the default http:80:5000). Other existing mappings
// are kept. changed is false when existing already matches the result.
func mergePorts(existing []string, toAdd []string) (merged []string, changed bool) {
	addHostKeys := make(map[string]struct{})
	for _, p := range toAdd {
		addHostKeys[portHostKey(p)] = struct{}{}
	}

	merged = []string{}
	for _, p := range existing {
		if _, ok := addHostKeys[portHostKey(p)]; !ok {
			merged = append(merged, p)
		}
	}

	seen := make(map[string]struct{})
	for _, p := range toAdd {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			merged = append(merged, p)
		}
	}

	existingLookup := sliceToLookupMap(existing)
	mergedLookup := sliceToLookupMap(merged)
	changed = len(existingLookup) != len(mergedLookup)
	for p := range mergedLookup {
		if _, ok := existingLookup[p]; !ok {
			changed = true
		}
	}

	return merged, changed
}
