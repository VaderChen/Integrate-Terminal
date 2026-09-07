package version

import "testing"

func TestUpdateVersionIncludesBuildLabel(t *testing.T) {
	previousProduct, previousBuild := Product, Build
	t.Cleanup(func() {
		Product, Build = previousProduct, previousBuild
	})

	Product = "1.26.0904"
	Build = "1529"
	if got, want := UpdateVersion(), "1.26.0904.1529"; got != want {
		t.Fatalf("UpdateVersion() = %q, want %q", got, want)
	}
	if got, want := DisplayVersion(), "1.26.0904 build 1529"; got != want {
		t.Fatalf("DisplayVersion() = %q, want %q", got, want)
	}
}

func TestUpdateVersionKeepsMarketingVersionForLegacyBuild(t *testing.T) {
	previousProduct, previousBuild := Product, Build
	t.Cleanup(func() {
		Product, Build = previousProduct, previousBuild
	})

	Product = "1.26.0904"
	Build = ""
	if got, want := UpdateVersion(), "1.26.0904"; got != want {
		t.Fatalf("UpdateVersion() = %q, want %q", got, want)
	}
}
