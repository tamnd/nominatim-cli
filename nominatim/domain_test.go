package nominatim

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network. The client's HTTP behaviour is
// covered in nominatim_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "nominatim" {
		t.Errorf("Scheme = %q, want nominatim", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "nominatim" {
		t.Errorf("Identity.Binary = %q, want nominatim", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ string }{
		{"Paris, France", "place"},
		{"48.8566,2.3522", "place"},
	}
	for _, tc := range cases {
		typ, _, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ {
			t.Errorf("Classify(%q) type = %q (err=%v), want %q", tc.in, typ, err, tc.typ)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("place", "Paris, France")
	if err != nil {
		t.Fatalf("Locate error: %v", err)
	}
	if got == "" {
		t.Error("Locate returned empty URL")
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown resource type")
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round-trip:
// a record mints to its URI and resolves back.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	got, err := h.ResolveOn("nominatim", "Paris")
	if err != nil {
		t.Fatalf("ResolveOn: %v", err)
	}
	if got.String() != "nominatim://place/Paris" {
		t.Errorf("ResolveOn = %q, want nominatim://place/Paris", got.String())
	}
}
