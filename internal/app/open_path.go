package app

import (
	"fmt"
	stdruntime "runtime"
)

func openCommandForPath(targetPath string) (string, []string, error) {
	switch stdruntime.GOOS {
	case "darwin":
		return "open", []string{targetPath}, nil
	case "linux":
		return "xdg-open", []string{targetPath}, nil
	case "windows":
		return "cmd", []string{"/c", "start", "", targetPath}, nil
	default:
		return "", nil, fmt.Errorf("open is not supported on %s", stdruntime.GOOS)
	}
}
