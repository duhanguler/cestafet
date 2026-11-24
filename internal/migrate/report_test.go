package migrate

import (
	"os"
	"testing"
)

// Looking at a cleanly closed old copy leaves no journal files beside it, so a
// "was anything written here?" check on the folder stays truthful.
func TestInspectLeavesNoJournalBesideAClosedCopy(t *testing.T) {
	c := makeCopy(t, "old", t0, "")
	for _, j := range []string{c.DB + "-wal", c.DB + "-shm"} {
		if exists(j) {
			t.Fatalf("fixture left %s", j)
		}
	}
	got := Inspect([]Copy{c})
	if len(got) != 1 || got[0].Corrupt != "" || got[0].Empty || !got[0].Activity.Equal(t0) {
		t.Fatalf("inspect %+v", got)
	}
	if _, err := Rank([]Copy{c}); err != nil {
		t.Fatal(err)
	}
	for _, j := range []string{c.DB + "-wal", c.DB + "-shm"} {
		if exists(j) {
			t.Fatalf("inspecting created %s", j)
		}
	}
}

// Inside an MSIX package the plain AppData path is redirected to the package's
// copy: the same file then appears under both names, and the package's name is
// the true one.
func TestRedirectedCandidateTakesTheRealCopysName(t *testing.T) {
	pkg := makeCopy(t, "Claude_pkg", t0, "")
	appdata := Copy{Name: "AppData", DB: pkg.DB + ".alias", Projects: pkg.Projects + ".alias", Redirectable: true}
	if err := os.Link(pkg.DB, appdata.DB); err != nil {
		t.Skip("hard links unsupported:", err)
	}
	got := Inspect([]Copy{appdata, pkg})
	if len(got) != 1 || got[0].Copy.Name != "Claude_pkg" || got[0].Copy.DB != pkg.DB {
		t.Fatalf("inspect %+v", got)
	}
	ranked, err := Rank([]Copy{appdata, pkg})
	if err != nil || len(ranked) != 1 || ranked[0].Copy.Name != "Claude_pkg" {
		t.Fatalf("rank %+v %v", ranked, err)
	}
}
