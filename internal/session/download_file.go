package session

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// downloadFile writes inside an already-open root, so symlink changes cannot
// redirect directory creation, the temporary file, or the final rename outside
// the selected destination. The existing file remains untouched until commit.
func downloadFile(root *os.Root, relativePath string, receive func(io.Writer) error) error {
	parent := filepath.Dir(relativePath)
	if err := root.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if info, err := root.Lstat(relativePath); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("download destination is not a regular file: %s", relativePath)
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	temporaryPath := filepath.Join(parent, ".integterm-download-"+uuid.NewString()+".part")
	file, err := root.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(temporaryPath)
	if err := receive(file); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return root.Rename(temporaryPath, relativePath)
}
