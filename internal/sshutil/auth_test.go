package sshutil

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/ssh"
)

func TestSignerFromPPK(t *testing.T) {
	for _, version := range []int{2, 3} {
		for _, test := range []struct {
			name, storedPassphrase, suppliedPassphrase string
			corrupt, wantError                         bool
			wantMessage                                string
		}{
			{name: "未加密且密語留空"},
			{name: "未加密忽略舊密語", suppliedPassphrase: "unused"},
			{name: "加密且密語正確", storedPassphrase: " passphrase ", suppliedPassphrase: " passphrase "},
			{name: "加密且密語留空", storedPassphrase: "passphrase", wantError: true},
			{name: "加密且密語錯誤", storedPassphrase: "passphrase", suppliedPassphrase: "wrong", wantError: true,
				wantMessage: "PPK 驗證失敗：密語不符或檔案損毀，請確認金鑰檔案與 PPK 密語"},
			{name: "加密且密語正確但內容損毀", storedPassphrase: "passphrase", suppliedPassphrase: "passphrase", corrupt: true, wantError: true,
				wantMessage: "PPK 驗證失敗：密語不符或檔案損毀，請確認金鑰檔案與 PPK 密語"},
			{name: "未加密但內容損毀", corrupt: true, wantError: true,
				wantMessage: "PPK 完整性驗證失敗：此檔案未加密，請檢查檔案是否已變動或損毀"},
		} {
			t.Run(fmt.Sprintf("v%d/%s", version, test.name), func(t *testing.T) {
				contents, publicKey := makeTestPPK(t, version, test.storedPassphrase)
				if test.corrupt {
					contents = strings.Replace(contents, "Comment: smoke", "Comment: modified", 1)
				}
				// 同時覆蓋 Windows 換行的 PPK 檔案。
				if version == 2 {
					contents = strings.ReplaceAll(contents, "\n", "\r\n")
				}
				signer, err := SignerFromPPK(writeTestPPK(t, contents), test.suppliedPassphrase)
				if test.wantError {
					if err == nil || signer != nil {
						t.Fatal("無效金鑰或錯誤密語不應產生 signer")
					}
					if test.wantMessage != "" && err.Error() != test.wantMessage {
						t.Fatalf("驗證失敗應顯示可理解的提示，不輸出原始雜湊: %v", err)
					}
					if test.storedPassphrase != "" && test.suppliedPassphrase == "" && !strings.Contains(err.Error(), "PPK 已加密（aes256-cbc）") {
						t.Fatalf("應明確說明此檔案已加密: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				message := []byte("PPK signing smoke test")
				signature, err := signer.Sign(rand.Reader, message)
				if err != nil {
					t.Fatal(err)
				}
				if err := publicKey.Verify(message, signature); err != nil {
					t.Fatalf("PPK 簽章驗證失敗: %v", err)
				}
			})
		}
	}
}

func TestSignerFromPPKMissingPrivateKey(t *testing.T) {
	contents, _ := makeTestPPK(t, 2, "")
	contents = strings.Split(contents, "Private-Lines:")[0]
	contents = strings.Replace(contents, "Encryption: none\n", "", 1)
	_, err := SignerFromPPK(writeTestPPK(t, contents), "")
	if err == nil || !strings.Contains(err.Error(), "不含私鑰") {
		t.Fatalf("只有公鑰的檔案應提示缺少私鑰，而非要求密語: %v", err)
	}
}

func TestUnencryptedPPKSSHSmoke(t *testing.T) {
	contents, publicKey := makeTestPPK(t, 3, "")
	signer, err := SignerFromPPK(writeTestPPK(t, contents), "")
	if err != nil {
		t.Fatal(err)
	}
	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(key.Marshal(), publicKey.Marshal()) {
			return nil, fmt.Errorf("非預期的測試公鑰")
		}
		return nil, nil
	}}
	serverConfig.AddHostKey(hostSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	serverResult := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverResult <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		server, _, _, err := ssh.NewServerConn(conn, serverConfig)
		if err == nil {
			_ = server.Close()
		}
		serverResult <- err
	}()
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	client, _, _, err := ssh.NewClientConn(conn, listener.Addr().String(), &ssh.ClientConfig{
		User: "smoke", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.FixedHostKey(hostSigner.PublicKey()),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := <-serverResult; err != nil {
		t.Fatal(err)
	}
}

func writeTestPPK(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "smoke.ppk")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// 測試金鑰於執行時產生，只存放在測試暫存目錄，不接觸使用者金鑰。
func makeTestPPK(t *testing.T, version int, passphrase string) (string, ssh.PublicKey) {
	t.Helper()
	pub, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	privateBlob := ssh.Marshal(struct{ Seed []byte }{privateKey.Seed()})
	encryption, derivation := "none", ""
	var cipherKey, macKey []byte
	iv := make([]byte, aes.BlockSize)
	if passphrase != "" {
		encryption = "aes256-cbc"
		privateBlob = append(privateBlob, make([]byte, (aes.BlockSize-len(privateBlob)%aes.BlockSize)%aes.BlockSize)...)
	}
	if version == 2 {
		mac := sha1.Sum([]byte("putty-private-key-file-mac-key" + passphrase))
		macKey = mac[:]
		for index := uint32(0); index < 2; index++ {
			input := binary.BigEndian.AppendUint32(nil, index)
			digest := sha1.Sum(append(input, []byte(passphrase)...))
			cipherKey = append(cipherKey, digest[:]...)
		}
		cipherKey = cipherKey[:32]
	} else if passphrase != "" {
		salt := []byte("smoke-test-salt")
		derived := argon2.IDKey([]byte(passphrase), salt, 1, 64, 1, 80)
		cipherKey, iv, macKey = derived[:32], derived[32:48], derived[48:]
		derivation = fmt.Sprintf("Key-Derivation: Argon2id\nArgon2-Memory: 64\nArgon2-Passes: 1\nArgon2-Parallelism: 1\nArgon2-Salt: %x\n", salt)
	}
	mac := hmac.New(sha256.New, macKey)
	if version == 2 {
		mac = hmac.New(sha1.New, macKey)
	}
	for _, field := range [][]byte{[]byte(ssh.KeyAlgoED25519), []byte(encryption), []byte("smoke"), publicKey.Marshal(), privateBlob} {
		_, _ = mac.Write(binary.BigEndian.AppendUint32(nil, uint32(len(field))))
		_, _ = mac.Write(field)
	}
	checksum := mac.Sum(nil)
	if passphrase != "" {
		block, err := aes.NewCipher(cipherKey)
		if err != nil {
			t.Fatal(err)
		}
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(privateBlob, privateBlob)
	}
	blobLines := func(name string, blob []byte) string {
		encoded := base64.StdEncoding.EncodeToString(blob)
		lines := fmt.Sprintf("%s-Lines: %d\n", name, (len(encoded)+63)/64)
		for len(encoded) > 64 {
			lines += encoded[:64] + "\n"
			encoded = encoded[64:]
		}
		return lines + encoded + "\n"
	}
	return fmt.Sprintf("PuTTY-User-Key-File-%d: %s\nEncryption: %s\nComment: smoke\n%s%s%sPrivate-MAC: %x\n",
		version, ssh.KeyAlgoED25519, encryption, blobLines("Public", publicKey.Marshal()), derivation,
		blobLines("Private", privateBlob), checksum), publicKey
}
