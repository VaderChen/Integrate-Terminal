package app

import (
	"fmt"
	"os"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ExportRestAPIDocsMarkdown() (string, error) {
	return a.exportMCPContractMarkdown(string(mcpContractNetwork), "integterm-skill.md")
}

func (a *App) ExportMCPContractMarkdown(contract string) (string, error) {
	filename := "integterm-mcp-network.md"
	if strings.EqualFold(strings.TrimSpace(contract), string(mcpContractLocal)) {
		filename = "integterm-mcp-local.md"
	}
	return a.exportMCPContractMarkdown(contract, filename)
}

func (a *App) exportMCPContractMarkdown(contract, defaultFilename string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("desktop context is not available")
	}

	markdown, err := a.GetMCPContractMarkdown(contract)
	if err != nil {
		return "", err
	}

	targetPath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "匯出 Markdown 文件",
		DefaultFilename: defaultFilename,
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Markdown File",
				Pattern:     "*.md",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(targetPath) == "" {
		return "", nil
	}

	if !strings.HasSuffix(strings.ToLower(targetPath), ".md") {
		targetPath += ".md"
	}

	if err := os.WriteFile(targetPath, []byte(markdown), 0o644); err != nil {
		return "", err
	}

	return targetPath, nil
}
