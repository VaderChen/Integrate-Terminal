// Package boundedlog 提供跨行程同步且限制磁碟容量的日誌追加。
package boundedlog

import (
	"fmt"
	"os"
)

// Append 保留完整新紀錄；超出容量時淘汰舊內容，再開始追加。
// 單筆超過容量時只保留尾端，不額外配置整份日誌或長駐檔案控制代碼。
func Append(path string, data []byte, maxBytes int64) (int, error) {
	if maxBytes <= 0 {
		return 0, fmt.Errorf("log size limit must be positive")
	}
	originalLength := len(data)
	if originalLength == 0 {
		return 0, nil
	}
	if int64(len(data)) > maxBytes {
		data = data[int64(len(data))-maxBytes:]
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	// 直接鎖住固定檔案；GUI 與背景服務不會各自持有舊的輪替檔案。
	if err := lockFile(file); err != nil {
		return 0, err
	}
	defer unlockFile(file)
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if info.Size() > maxBytes-int64(len(data)) {
		if err := file.Truncate(0); err != nil {
			return 0, err
		}
	}
	n, err := file.Write(data)
	return originalLength - len(data) + n, err
}
