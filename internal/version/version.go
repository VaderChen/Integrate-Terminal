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

func ProductVersion() string {
	var cfg configFile
	if err := json.Unmarshal(versionJSON, &cfg); err != nil {
		return "1.00.00"
	}
	version := strings.TrimSpace(cfg.ProductVersion)
	if version == "" {
		version = "1.00.00"
	}
	return version
}

func Current() string { return "IntegTERM " + ProductVersion() }

func UpdateVersion() string {
	var cfg configFile
	_ = json.Unmarshal(versionJSON, &cfg)
	if strings.TrimSpace(cfg.Build) != "" {
		return ProductVersion() + "." + strings.TrimSpace(cfg.Build)
	}
	return ProductVersion()
}
