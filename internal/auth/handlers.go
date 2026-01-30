package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// StatusResp 认证状态
type StatusResp struct {
	NeedSetup bool `json:"need_setup"` // 未设置主密码，需首次设置
	LoggedIn  bool `json:"logged_in"`  // 已登录
}

// Status 返回当前认证状态。
// 重要：NeedSetup 仅在「从未设置过主密码」时为 true（本机不存在 .auth_hash 文件）；
// 一旦设置过主密码，之后永远只返回 NeedSetup: false，前端只显示登录框，不再显示设置密码框。
func Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	hasPwd, err := HasPassword()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !hasPwd {
		// 仅首次：本机尚未存在主密码哈希文件，需设置主密码
		_ = json.NewEncoder(w).Encode(StatusResp{NeedSetup: true, LoggedIn: false})
		return
	}
	// 已设置过主密码：仅需登录，不再出现设置密码界面
	_, ok := getSession(r)
	_ = json.NewEncoder(w).Encode(StatusResp{NeedSetup: false, LoggedIn: ok})
}

// SetupReq 首次设置主密码
type SetupReq struct {
	Password string `json:"password"`
	Confirm  string `json:"confirm"`
}

// Setup 仅首次可用：设置主密码并登录。一旦本机已存在主密码哈希，此接口拒绝再次设置。
func Setup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hasPwd, err := HasPassword()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if hasPwd {
		// 已设置过主密码，不允许再次走设置流程，应使用登录接口
		http.Error(w, "already set", http.StatusBadRequest)
		return
	}
	var req SetupReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	req.Confirm = strings.TrimSpace(req.Confirm)
	if req.Password != req.Confirm {
		http.Error(w, "两次密码不一致", http.StatusBadRequest)
		return
	}
	if err := SetPassword(req.Password); err != nil {
		if err == ErrPasswordTooShort {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 设置成功后不创建会话，让用户跳转到登录页用新密码登录
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// LoginReq 登录
type LoginReq struct {
	Password string `json:"password"`
}

// Login 验证主密码并创建会话
func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	ok, err := VerifyPassword(strings.TrimSpace(req.Password))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "密码错误", http.StatusUnauthorized)
		return
	}
	_, err = createSession(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Logout 登出
func Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	destroySession(w, r)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ResetReq 重设主密码（需已登录）
type ResetReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	Confirm         string `json:"confirm"`
}

// Reset 重设主密码：校验当前密码后写入新密码哈希
func Reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ResetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	cur := strings.TrimSpace(req.CurrentPassword)
	newPwd := strings.TrimSpace(req.NewPassword)
	confirm := strings.TrimSpace(req.Confirm)
	if cur == "" {
		http.Error(w, "请输入当前密码", http.StatusBadRequest)
		return
	}
	ok, err := VerifyPassword(cur)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "当前密码错误", http.StatusUnauthorized)
		return
	}
	if newPwd != confirm {
		http.Error(w, "两次新密码不一致", http.StatusBadRequest)
		return
	}
	if err := SetPassword(newPwd); err != nil {
		if err == ErrPasswordTooShort {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
