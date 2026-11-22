package main

import (
	"runtime/debug"
	"strings"
)

// version is set at build time with -ldflags "-X main.version=..." (it must stay
// a constant-initialised string for -X to apply). Use appVersion to read it.
var version = "dev"

// appVersion is the ldflags version or, for a `go install ...@vX.Y.Z` build that
// has no ldflags, the module's tagged version.
func appVersion() string { return moduleVersion(version, readModuleVersion) }

func readModuleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

// moduleVersion keeps an ldflags version, else uses the module's tagged
// version without its "v" prefix. Untagged builds ("(devel)", pseudo-versions)
// stay "dev" so they are never compared with a plugin version.
func moduleVersion(ldflags string, read func() string) string {
	if ldflags != "dev" {
		return ldflags
	}
	v := strings.TrimPrefix(read(), "v")
	if v == "" || v == "(devel)" || strings.Count(v, "-") > 0 || strings.Contains(v, "+") {
		return ldflags
	}
	return v
}
