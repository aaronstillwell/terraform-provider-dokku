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

// conflictingPorts returns the mappings in existing that share a
// scheme:hostPort with a mapping in desired, excluding exact matches. These
// must be removed before the desired mappings are added, otherwise dokku keeps
// both (e.g the default http:80:5000 alongside http:80:8080).
func conflictingPorts(existing []string, desired []string) []string {
	desiredLookup := sliceToLookupMap(desired)
	desiredHostKeys := make(map[string]struct{})
	for _, p := range desired {
		desiredHostKeys[portHostKey(p)] = struct{}{}
	}

	conflicts := []string{}
	for _, p := range existing {
		if _, ok := desiredLookup[p]; ok {
			continue
		}
		if _, ok := desiredHostKeys[portHostKey(p)]; ok {
			conflicts = append(conflicts, p)
		}
	}
	return conflicts
}
