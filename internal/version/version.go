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

type BuildInfo struct {
	Commit     string `json:"commit"`
	Tag        string `json:"tag"`
	BuildState string `json:"buildState"`
	SourceURL  string `json:"sourceUrl"`
}

var (
	Product    = ""
	Commit     = "unknown"
	Tag        = "untagged"
	BuildState = "unknown"
	SourceURL  = "https://github.com/VaderChen/Integrate-Terminal"
)

func Current() string {
	return "IntegTERM " + ProductVersion()
}

func ProductVersion() string {
	if productVersion := strings.TrimSpace(Product); productVersion != "" {
		return productVersion
	}
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

func Metadata() BuildInfo {
	return BuildInfo{
		Commit:     valueOrDefault(Commit, "unknown"),
		Tag:        valueOrDefault(Tag, "untagged"),
		BuildState: valueOrDefault(BuildState, "unknown"),
		SourceURL:  valueOrDefault(SourceURL, "https://github.com/VaderChen/Integrate-Terminal"),
	}
}

func valueOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
