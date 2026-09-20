package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"sort"
	"strings"

	"IntegTERM/internal/model"
)

func filterHiddenEntries(entries []model.FileEntry, showHidden bool) []model.FileEntry {
	if showHidden {
		return entries
	}
	filtered := make([]model.FileEntry, 0, len(entries))
	for _, entry := range entries {
		if len(entry.Name) > 0 && entry.Name[0] == '.' {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func resolveAppDataDir() string {
	baseDir, err := os.UserConfigDir()
	if err == nil && strings.TrimSpace(baseDir) != "" {
		return filepath.Join(baseDir, "IntegTERM")
	}

	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".integterm")
	}

	return "data"
}

func defaultLocalPath() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}

	return filepath.Clean(".")
}

func migrateLegacyDataDir(targetDir string) {
	legacyDir := "data"
	if filepath.Clean(targetDir) == filepath.Clean(legacyDir) {
		return
	}

	if _, err := os.Stat(legacyDir); err != nil {
		return
	}

	_ = os.MkdirAll(targetDir, 0o700)
	for _, name := range []string{"sites.json", "tabs.json", "config.json"} {
		sourcePath := filepath.Join(legacyDir, name)
		destPath := filepath.Join(targetDir, name)

		if _, err := os.Stat(sourcePath); err != nil {
			continue
		}
		if _, err := os.Stat(destPath); err == nil {
			continue
		}

		data, err := os.ReadFile(sourcePath)
		if err != nil {
			continue
		}
		_ = os.WriteFile(destPath, data, 0o600)
	}
}

type systemProfilerFontPayload struct {
	Fonts []struct {
		Typefaces []struct {
			Family string `json:"family"`
		} `json:"typefaces"`
	} `json:"SPFontsDataType"`
}

func listSystemFonts() ([]string, error) {
	if stdruntime.GOOS != "darwin" {
		return fallbackTerminalFonts(), nil
	}

	output, err := exec.Command("system_profiler", "SPFontsDataType", "-json").Output()
	if err != nil {
		return nil, err
	}

	var payload systemProfilerFontPayload
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	fonts := make([]string, 0)
	for _, font := range payload.Fonts {
		for _, typeface := range font.Typefaces {
			family := strings.TrimSpace(typeface.Family)
			if family == "" {
				continue
			}
			if !isTerminalFontFamily(family) {
				continue
			}
			if _, exists := seen[family]; exists {
				continue
			}
			seen[family] = struct{}{}
			fonts = append(fonts, family)
		}
	}

	if len(fonts) == 0 {
		return fallbackTerminalFonts(), nil
	}

	sort.Strings(fonts)
	return fonts, nil
}

func fallbackTerminalFonts() []string {
	return []string{
		"SF Mono",
		"Menlo",
		"Monaco",
		"Cascadia Mono",
		"Consolas",
		"JetBrains Mono",
		"Fira Code",
		"Source Code Pro",
		"Hack",
		"Iosevka",
	}
}

func isTerminalFontFamily(family string) bool {
	name := strings.ToLower(strings.TrimSpace(family))
	if name == "" {
		return false
	}

	keywords := []string{
		"mono",
		"code",
		"console",
		"terminal",
		"fixed",
		"typewriter",
	}
	for _, keyword := range keywords {
		if strings.Contains(name, keyword) {
			return true
		}
	}

	exactMatches := map[string]struct{}{
		"menlo":          {},
		"monaco":         {},
		"consolas":       {},
		"courier":        {},
		"courier new":    {},
		"andale mono":    {},
		"lucida console": {},
		"pt mono":        {},
		"input":          {},
		"pragmatapro":    {},
		"berkeley mono":  {},
		"operator mono":  {},
		"ubuntu mono":    {},
		"comic code":     {},
		"recursive mono": {},
		"departure mono": {},
		"iosevka term":   {},
		"iosevka fixed":  {},
		"apl385 unicode": {},
		"apl333":         {},
	}
	_, ok := exactMatches[name]
	return ok
}
