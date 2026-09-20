package app

import (
	"fmt"
	"os"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ExportRestAPIDocsMarkdown() (string, error) {
	ctx := a.appContext()
	if ctx == nil {
		return "", fmt.Errorf("desktop context is not available")
	}

	markdown, err := a.GetRestAPIDocsMarkdown()
	if err != nil {
		return "", err
	}

	targetPath, err := wailsruntime.SaveFileDialog(ctx, wailsruntime.SaveDialogOptions{
		Title:           "匯出 Markdown 文件",
		DefaultFilename: "integterm-mcp.md",
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

	if err := os.WriteFile(targetPath, []byte(markdown), 0o600); err != nil {
		return "", err
	}

	if err := os.Chmod(targetPath, 0o600); err != nil {
		return "", err
	}
	return targetPath, nil
}
