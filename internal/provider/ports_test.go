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

func TestMergePorts(t *testing.T) {
	cases := []struct {
		name        string
		existing    []string
		toAdd       []string
		want        []string
		wantChanged bool
	}{
		{
			name:        "replaces mapping on same scheme and host port",
			existing:    []string{"http:80:5000", "https:443:5000", "tcp:25:3000"},
			toAdd:       []string{"http:80:8080"},
			want:        []string{"https:443:5000", "tcp:25:3000", "http:80:8080"},
			wantChanged: true,
		},
		{
			name:        "keeps existing exact match",
			existing:    []string{"http:80:5000", "tcp:25:3000"},
			toAdd:       []string{"tcp:25:3000"},
			want:        []string{"http:80:5000", "tcp:25:3000"},
			wantChanged: false,
		},
		{
			name:        "same host port on another scheme is not a conflict",
			existing:    []string{"http:8080:5000"},
			toAdd:       []string{"https:8080:5000"},
			want:        []string{"http:8080:5000", "https:8080:5000"},
			wantChanged: true,
		},
		{
			name:        "no existing ports",
			existing:    []string{},
			toAdd:       []string{"http:80:8080", "http:80:8080"},
			want:        []string{"http:80:8080"},
			wantChanged: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, changed := mergePorts(c.existing, c.toAdd)

			if !slices.Equal(c.want, got) {
				t.Errorf("mergePorts() = %q, want %q", got, c.want)
			}
			if changed != c.wantChanged {
				t.Errorf("mergePorts() changed = %v, want %v", changed, c.wantChanged)
			}
		})
	}
}
