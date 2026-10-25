package cards

import "testing"

func TestThemeCatalogIsIndependent(t *testing.T) {
	first := ThemeCatalog()
	first["stats"]["radical"]["bg_color"] = "000000"
	delete(first["streak"], "radical")
	second := ThemeCatalog()
	if second["stats"]["radical"]["bg_color"] != "141321" || second["streak"]["radical"]["background"] != "#141321" {
		t.Fatal("mutating the builder catalog must not alter renderer themes")
	}
}
