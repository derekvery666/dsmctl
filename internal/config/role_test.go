package config

import "testing"

func TestNormalizeRoleAndManaged(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", true},
		{"managed", "", true},
		{"Managed", "", true},
		{" target ", ProfileRoleTarget, true},
		{"TARGET", ProfileRoleTarget, true},
		{"bogus", "", false},
	}
	for _, tc := range cases {
		got, err := NormalizeRole(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Fatalf("NormalizeRole(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Fatalf("NormalizeRole(%q) expected an error", tc.in)
		}
	}
	if !(Profile{}).Managed() {
		t.Fatal("an empty role must be managed")
	}
	if !(Profile{Role: "managed"}).Managed() {
		t.Fatal("the managed role must be managed")
	}
	if (Profile{Role: ProfileRoleTarget}).Managed() {
		t.Fatal("the target role must not be managed")
	}
}
