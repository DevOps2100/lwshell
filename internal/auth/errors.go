package auth

import "errors"

var (
	ErrPasswordTooShort = errors.New("密码至少 6 位")
)
