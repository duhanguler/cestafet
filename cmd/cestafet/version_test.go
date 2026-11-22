package main

import "testing"

func TestModuleVersion(t *testing.T) {
	cases := []struct{ ldflags, module, want string }{
		{"0.2.0", "v0.1.1", "0.2.0"},
		{"dev", "v0.1.1", "0.1.1"},
		{"dev", "(devel)", "dev"},
		{"dev", "", "dev"},
		{"dev", "v0.1.2-0.20261122103000-3743ffe0a1b2", "dev"},
		{"dev", "v0.1.1+dirty", "dev"},
	}
	for _, c := range cases {
		if got := moduleVersion(c.ldflags, func() string { return c.module }); got != c.want {
			t.Errorf("moduleVersion(%q, %q) = %q, want %q", c.ldflags, c.module, got, c.want)
		}
	}
}
