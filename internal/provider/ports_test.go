package provider

import (
	"slices"
	"testing"
)

func TestPortHostKey(t *testing.T) {
	cases := map[string]string{
		"http:80:5000":  "http:80",
		"https:443:80":  "https:443",
		" tcp:25:3000 ": "tcp:25",
	}

	for in, want := range cases {
		if got := portHostKey(in); got != want {
			t.Errorf("portHostKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConflictingPorts(t *testing.T) {
	existing := []string{"http:80:5000", "https:443:5000", "tcp:25:3000", "http:8080:5000"}
	desired := []string{"http:80:8080", "tcp:25:3000", "https:8443:5000"}

	want := []string{"http:80:5000"}
	got := conflictingPorts(existing, desired)

	if !slices.Equal(want, got) {
		t.Errorf("conflictingPorts() = %q, want %q", got, want)
	}
}

func TestConflictingPortsNoExisting(t *testing.T) {
	got := conflictingPorts([]string{}, []string{"http:80:8080"})

	if len(got) != 0 {
		t.Errorf("conflictingPorts() = %q, want none", got)
	}
}
