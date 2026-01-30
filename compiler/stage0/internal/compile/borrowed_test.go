package compile

import "testing"

func TestBorrowedTypeNamesStage2Loader(t *testing.T) {
	// These types are value-passed within the stage2 compiler and contain
	// owning fields (String/[String]/Program). They must be treated as
	// borrowed to avoid double-drop until move tracking is complete.
	names := []string{
		"ImportSpec",
		"DepSpec",
		"ManifestInfo",
		"WorkspaceInfo",
		"PackageInfo",
		"FileUnit",
		"LoadCtx",
		"LoadResult",
	}
	for _, name := range names {
		if !isBorrowedTypeName(name) {
			t.Fatalf("expected borrowed type: %s", name)
		}
	}
}
