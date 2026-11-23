package datadir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyCopiesRealFirstThenPackagesSorted(t *testing.T) {
	prof := t.TempDir()
	for _, p := range []string{"Claude_b", "Claude_a", "Other_x"} {
		if err := os.MkdirAll(filepath.Join(prof, "AppData", "Local", "Packages", p), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	got := LegacyCopies(prof)
	if len(got) != 3 || got[0].Name != "AppData" || got[1].Name != "Claude_a" || got[2].Name != "Claude_b" {
		t.Fatalf("copies %+v", got)
	}
	want := filepath.Join(prof, "AppData", "Local", "Packages", "Claude_a", "LocalCache", "Roaming", "cestafet", "projects.toml")
	if got[1].Projects != want {
		t.Fatalf("projects path %q", got[1].Projects)
	}
}
