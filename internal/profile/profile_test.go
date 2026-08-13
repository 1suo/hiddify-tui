package profile

import "testing"

func TestDefaultName(t *testing.T) {
	cases := []struct {
		content string
		want    string
	}{
		{`{"outbounds":[{"type":"vless","tag":"vless § 0"}]}`, "vless § 0"},
		{`{"outbounds":[]}`, "Local"},
		{`vless://uuid@host:443?security=reality#MyNode`, "MyNode"},
		{`vless://uuid@host:443`, "Local"},
		{`not a config`, "Local"},
	}
	for _, c := range cases {
		if got := DefaultName(c.content); got != c.want {
			t.Errorf("DefaultName(%q) = %q, want %q", c.content, got, c.want)
		}
	}
}
