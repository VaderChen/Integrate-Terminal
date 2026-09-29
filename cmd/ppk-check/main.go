// ppk-check 直接驗證指定的 PPK 檔案，不使用保存的副本或連線到遠端站台。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"IntegTERM/internal/sshutil"

	"github.com/kayrus/putty"
)

func main() {
	path := flag.String("file", "", "PPK 檔案路徑；密語僅由標準輸入讀取")
	flag.Parse()
	if *path == "" {
		fmt.Fprintln(os.Stderr, "請指定 -file；請使用 scripts/check-ppk.command 安全輸入密語")
		os.Exit(2)
	}
	if err := check(*path, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(path string, input io.Reader, output io.Writer) error {
	key, err := putty.NewFromFile(path)
	if err != nil {
		return fmt.Errorf("讀取 PPK 失敗：%w", err)
	}
	fmt.Fprintf(output, "PPK 版本：%d\n金鑰類型：%s\n加密方式：%s\n", key.Version, key.Algo, key.Encryption)
	// 不裁切空白或換行；密語必須保留使用者輸入的原始位元組。
	password, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil {
		return fmt.Errorf("讀取密語失敗：%w", err)
	}
	defer clear(password)
	if len(password) > 65536 {
		return fmt.Errorf("密語長度超過診斷工具限制")
	}
	if _, err := sshutil.SignerFromPPK(path, string(password)); err != nil {
		return fmt.Errorf("直接檔案驗證未通過：%w", err)
	}
	fmt.Fprintln(output, "驗證成功：指定檔案與輸入密語可正確載入。若 App 仍失敗，請檢查執行版本、保存的副本與站台／分頁設定。")
	return nil
}
