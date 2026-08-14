package app

import wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

func (a *App) ResetWindowToDefaultScale() error {
	if a.ctx == nil {
		return nil
	}

	selection, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "重設視窗比例",
		Message:       "要將整個視窗重設回 1:1 預設比例嗎？",
		Buttons:       []string{"取消", "確認"},
		DefaultButton: "確認",
		CancelButton:  "取消",
	})
	if err != nil {
		return err
	}
	if selection != "確認" {
		return nil
	}

	const defaultWidth = 1440
	const defaultHeight = 920

	wailsruntime.WindowSetSize(a.ctx, defaultWidth, defaultHeight)
	wailsruntime.WindowCenter(a.ctx)

	a.config.WindowWidth = defaultWidth
	a.config.WindowHeight = defaultHeight
	a.config.WindowX = 0
	a.config.WindowY = 0
	return a.store.SaveConfig(a.config)
}
