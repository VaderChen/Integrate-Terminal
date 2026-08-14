package version

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed version.json
var versionJSON []byte

type configFile struct {
	ProductVersion string `json:"productVersion"`
}

func Current() string {
	var cfg configFile
	if err := json.Unmarshal(versionJSON, &cfg); err != nil {
		return "IntegTERM 1.00.00"
	}
	version := strings.TrimSpace(cfg.ProductVersion)
	if version == "" {
		version = "1.00.00"
	}
	return "IntegTERM " + version
}
