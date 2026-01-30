package auth

import (
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

func authDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lwshell"), nil
}

func hashPath() (string, error) {
	dir, err := authDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".auth_hash"), nil
}

// HasPassword 是否已设置主密码（存在哈希文件）
func HasPassword() (bool, error) {
	p, err := hashPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// SetPassword 设置主密码（写入 bcrypt 哈希，仅后端存储）
func SetPassword(password string) error {
	if len(password) < 6 {
		return ErrPasswordTooShort
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}
	dir, err := authDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	p, _ := hashPath()
	return os.WriteFile(p, hash, 0600)
}

// VerifyPassword 验证主密码
func VerifyPassword(password string) (bool, error) {
	p, err := hashPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	err = bcrypt.CompareHashAndPassword(data, []byte(password))
	return err == nil, nil
}
