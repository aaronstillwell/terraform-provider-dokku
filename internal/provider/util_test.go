package provider

import (
	"maps"
	"slices"
	"testing"
)

func TestInterfaceSliceToStrSlice(t *testing.T) {
	want := []string{"foo", "bar"}

	got := interfaceSliceToStrSlice([]any{"foo", "bar"})

	if !slices.Equal(want, got) {
		t.Errorf("interfaceSliceToStrSlice() = %q, want %q", got, want)
	}
}

func TestParseKeyValues(t *testing.T) {
	lines := []string{
		"       Nginx bind address ipv4:                          ",
		"       Nginx bind address ipv6:       2001:0db8:0000:0000:0000:ff00:0042:8329",
		"       Nginx computed bind address ipv6: ::",
		"       Nginx global bind address ipv6: 2001:db8::1",
		"       Nginx access log format:       $remote_addr - [$time_local]",
		"",
		"       not a key value line",
	}

	want := map[string]string{
		"Nginx bind address ipv4":          "",
		"Nginx bind address ipv6":          "2001:0db8:0000:0000:0000:ff00:0042:8329",
		"Nginx computed bind address ipv6": "::",
		"Nginx global bind address ipv6":   "2001:db8::1",
		"Nginx access log format":          "$remote_addr - [$time_local]",
	}

	got := parseKeyValues(lines)

	if !maps.Equal(want, got) {
		t.Errorf("parseKeyValues() = %q, want %q", got, want)
	}
}
