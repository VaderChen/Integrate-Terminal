package version

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed version.json
var versionJSON []byte

type configFile struct {
	Build          string `json:"build"`
	ProductVersion string `json:"productVersion"`
}

type versionInfo struct {
	product string
	display string
	update  string
}

// 版本資料在編譯時內嵌且不變；啟動時解析一次，供托盤與更新檢查共用。
var currentInfo = parseVersionInfo(versionJSON)

func parseVersionInfo(data []byte) versionInfo {
	var cfg configFile
	err := json.Unmarshal(data, &cfg)
	version := strings.TrimSpace(cfg.ProductVersion)
	if err != nil || version == "" {
		version = "1.00.00"
	}
	update := version
	if build := strings.TrimSpace(cfg.Build); build != "" {
		update += "." + build
	}
	return versionInfo{product: version, display: "IntegTERM " + version, update: update}
}

func ProductVersion() string { return currentInfo.product }

func Current() string { return currentInfo.display }

func UpdateVersion() string { return currentInfo.update }
