package version

import "testing"

func TestVersionInfoKeepsDefaultsAndBuildFormatting(t *testing.T) {
	cases := []struct {
		data, product, update string
	}{
		{`{"productVersion":" 1.26.0904 ","build":" 1530 "}`, "1.26.0904", "1.26.0904.1530"},
		{`{"productVersion":"1.26.0904"}`, "1.26.0904", "1.26.0904"},
		{`{"productVersion":" ","build":"42"}`, "1.00.00", "1.00.00.42"},
		{`{"productVersion":12,"build":"42"}`, "1.00.00", "1.00.00.42"},
		{`{`, "1.00.00", "1.00.00"},
	}
	for _, test := range cases {
		info := parseVersionInfo([]byte(test.data))
		if info.product != test.product || info.display != "IntegTERM "+test.product || info.update != test.update {
			t.Fatalf("版本解析 %s = %+v", test.data, info)
		}
	}
	info := parseVersionInfo(versionJSON)
	if ProductVersion() != info.product || Current() != info.display || UpdateVersion() != info.update {
		t.Fatal("快取與內嵌版本資料不符")
	}
}
