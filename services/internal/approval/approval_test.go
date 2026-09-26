package approval

import (
	"reflect"
	"testing"
)

func TestEvaluate(t *testing.T) {
	allow := []string{"example.com", "*.brand.in"}
	cases := []struct {
		name string
		p    Policy
		c    Change
		want []string
	}{
		{"off", Policy{Mode: ModeOff}, Change{URLs: []string{"https://evil.test"}}, []string{}},
		{"inside allowlist", Policy{Mode: ModeOutsideAllowlist, AllowedHosts: allow},
			Change{URLs: []string{"https://example.com/a", "https://shop.brand.in/x", "mailto:a@b.c"}}, []string{}},
		{"rule destination outside", Policy{Mode: ModeOutsideAllowlist, AllowedHosts: allow},
			Change{URLs: []string{"https://example.com/a", "https://Other.test/b"}}, []string{ReasonHostNotAllowlisted}},
		{"allowlist mode without a list", Policy{Mode: ModeOutsideAllowlist}, Change{URLs: []string{"https://x.test"}}, []string{}},
		{"all destination changes: update", Policy{Mode: ModeAllDestinationChanges}, Change{URLs: []string{"https://example.com"}},
			[]string{ReasonAllDestination}},
		{"all destination changes: new code", Policy{Mode: ModeAllDestinationChanges}, Change{NewCode: true}, []string{}},
		{"all changes: new code", Policy{Mode: ModeAllChanges}, Change{NewCode: true}, []string{ReasonNewCode}},
		{"all changes: update", Policy{Mode: ModeAllChanges}, Change{}, []string{ReasonAllDestination}},
	}
	for _, tc := range cases {
		got := Evaluate(tc.p, tc.c)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestHostsAndDescribe(t *testing.T) {
	h := Hosts([]string{"https://A.example/x", "http://a.example/y", "tel:+911234", "not a url", "https://b.example"})
	if !reflect.DeepEqual(h, []string{"a.example", "b.example"}) {
		t.Fatalf("hosts: %v", h)
	}
	d := Describe([]string{ReasonHostNotAllowlisted, ReasonNewCode}, []string{"example.com"})
	if len(d) != 2 || d[0] != "the destination is outside the workspace's approved domains (example.com)" {
		t.Fatalf("describe: %v", d)
	}
}
