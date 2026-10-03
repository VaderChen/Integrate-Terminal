package updater

import (
	"reflect"
	"strings"
	"testing"
)

func TestSelectAssetKeepsScoreNameAndValidation(t *testing.T) {
	asset := func(name string) githubAsset {
		return githubAsset{Name: name, BrowserDownloadURL: "https://example.invalid/" + name, Size: 12, Digest: "sha256:" + strings.Repeat("a", 64)}
	}
	best := asset("IntegTERM-A-macos-arm64.dmg")
	bad := asset("IntegTERM-0-macos-arm64.dmg")
	bad.Digest = "invalid"
	assets := []githubAsset{asset("IntegTERM-macos-arm64.zip"), asset("IntegTERM-linux-arm64.deb"), asset("IntegTERM-B-macos-arm64.dmg"), best, bad}
	for i := 0; i < len(assets); i++ {
		got, ok := selectAsset(assets, "darwin", "arm64")
		if !ok || !reflect.DeepEqual(got, best) {
			t.Fatalf("選擇結果不符：%+v %t", got, ok)
		}
		assets = append(assets[1:], assets[0])
	}
	if _, ok := selectAsset([]githubAsset{bad}, "darwin", "arm64"); ok {
		t.Fatal("接受無效摘要")
	}
	if _, ok := selectAsset(nil, "darwin", "arm64"); ok {
		t.Fatal("空清單仍回報安裝檔")
	}
}

func TestNormalizedVersionKeepsAcceptedAndRejectedForms(t *testing.T) {
	for input, want := range map[string]string{" v1.26.0904+build ": "1.26.0904", "V 1.02-rc1": "1.02", "01.002.0003": "01.002.0003"} {
		if got, err := normalizedVersion(input); err != nil || got != want {
			t.Fatalf("版本 %q = %q %v", input, got, err)
		}
	}
	for _, input := range []string{"", " ", "v", "1..2", ".1", "1.", "1.a"} {
		if _, err := normalizedVersion(input); err == nil {
			t.Fatalf("接受無效版本 %q", input)
		}
	}
}
