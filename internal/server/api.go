package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"lwshell/internal/config"
	"lwshell/internal/models"
)

// GroupResp 分组（不含密码）
type GroupResp struct {
	Name    string         `json:"name"`
	Servers []ServerResp    `json:"servers"`
}

// ServerResp 对外暴露的服务器信息（不含密码）
type ServerResp struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	User    string `json:"user"`
	KeyPath string `json:"key_path,omitempty"`
	Group   string `json:"group"`
}

// ConnectReq POST /api/connect 请求体
type ConnectReq struct {
	ID string `json:"id"`
}

func groupsFromConfig(cfg *models.Config) []GroupResp {
	m := make(map[string][]ServerResp)
	for _, s := range cfg.Servers {
		g := s.Group
		if g == "" {
			g = "未分组"
		}
		m[g] = append(m[g], ServerResp{
			ID:      s.ID,
			Name:    s.Name,
			Host:    s.Host,
			Port:    s.Port,
			User:    s.User,
			KeyPath: s.KeyPath,
			Group:   s.Group,
		})
	}
	names := []string{}
	for k := range m {
		if k != "未分组" {
			names = append(names, k)
		}
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	names = append(names, "未分组")
	out := make([]GroupResp, 0, len(names))
	for _, n := range names {
		if _, ok := m[n]; ok {
			out = append(out, GroupResp{Name: n, Servers: m[n]})
		}
	}
	return out
}

// ServersAPI 统一处理 /api/servers 与 /api/servers/:id
func ServersAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/api/servers" {
		switch r.Method {
		case http.MethodGet:
			GetServers(w, r)
			return
		case http.MethodPost:
			CreateServer(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(path, "/api/servers/") {
		id := strings.TrimPrefix(path, "/api/servers/")
		if id == "" {
			http.Error(w, "missing server id", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			UpdateServer(w, r, id)
			return
		case http.MethodDelete:
			DeleteServer(w, r, id)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.NotFound(w, r)
}

// GetServers 返回分组后的服务器列表（不含密码）
func GetServers(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"groups": groupsFromConfig(cfg)})
}

// ServerBody 创建/编辑时的请求体（密码可选）
// 编辑时 Password 为 nil 表示不修改原密码，空字符串表示清空
type ServerBody struct {
	Name     string  `json:"name"`
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	User     string  `json:"user"`
	Password *string `json:"password,omitempty"`
	KeyPath  string  `json:"key_path"`
	Group    string  `json:"group"`
}

func nextID(servers []models.Server) string {
	max := 0
	for _, s := range servers {
		if s.ID != "" {
			n, _ := strconv.Atoi(s.ID)
			if n > max {
				max = n
			}
		}
	}
	return strconv.Itoa(max + 1)
}

// CreateServer 添加服务器
func CreateServer(w http.ResponseWriter, r *http.Request) {
	var body ServerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Host = strings.TrimSpace(body.Host)
	body.User = strings.TrimSpace(body.User)
	if body.Name == "" || body.Host == "" || body.User == "" {
		http.Error(w, "name, host, user required", http.StatusBadRequest)
		return
	}
	if body.Port <= 0 {
		body.Port = 22
	}
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pwd := ""
	if body.Password != nil {
		pwd = strings.TrimSpace(*body.Password)
	}
	s := models.Server{
		ID:       nextID(cfg.Servers),
		Name:     body.Name,
		Host:     body.Host,
		Port:     body.Port,
		User:     body.User,
		Password: pwd,
		KeyPath:  strings.TrimSpace(body.KeyPath),
		Group:    strings.TrimSpace(body.Group),
	}
	cfg.Servers = append(cfg.Servers, s)
	if err := config.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": s.ID})
}

// UpdateServer 编辑服务器
func UpdateServer(w http.ResponseWriter, r *http.Request, id string) {
	var body ServerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Host = strings.TrimSpace(body.Host)
	body.User = strings.TrimSpace(body.User)
	if body.Name == "" || body.Host == "" || body.User == "" {
		http.Error(w, "name, host, user required", http.StatusBadRequest)
		return
	}
	if body.Port <= 0 {
		body.Port = 22
	}
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	found := false
	for i := range cfg.Servers {
		if cfg.Servers[i].ID == id {
			cfg.Servers[i].Name = body.Name
			cfg.Servers[i].Host = body.Host
			cfg.Servers[i].Port = body.Port
			cfg.Servers[i].User = body.User
			if body.Password != nil {
				cfg.Servers[i].Password = strings.TrimSpace(*body.Password)
			}
			cfg.Servers[i].KeyPath = strings.TrimSpace(body.KeyPath)
			cfg.Servers[i].Group = strings.TrimSpace(body.Group)
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}
	if err := config.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// DeleteServer 删除服务器
func DeleteServer(w http.ResponseWriter, r *http.Request, id string) {
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	newList := make([]models.Server, 0, len(cfg.Servers))
	for _, s := range cfg.Servers {
		if s.ID != id {
			newList = append(newList, s)
		}
	}
	if len(newList) == len(cfg.Servers) {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}
	cfg.Servers = newList
	if err := config.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Connect 在新终端窗口连接指定服务器（仅 macOS 用 osascript 打开 Terminal）
func Connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ConnectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "invalid body, need {\"id\":\"...\"}", http.StatusBadRequest)
		return
	}
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var target *models.Server
	for i := range cfg.Servers {
		if cfg.Servers[i].ID == req.ID {
			target = &cfg.Servers[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		http.Error(w, "cannot get executable path", http.StatusInternalServerError)
		return
	}
	// 路径含空格时用单引号包裹，便于 Terminal 正确解析
	connectCmd := "'" + escapeSingleQuotes(exe) + "' --connect-id=" + req.ID
	if runtime.GOOS == "darwin" {
		// 新开 Terminal 窗口执行：当前二进制 --connect-id=ID
		script := `tell application "Terminal" to do script "` + escapeAppleScript(connectCmd) + `"`
		if err := exec.Command("osascript", "-e", script).Run(); err != nil {
			http.Error(w, "failed to open terminal: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Linux/Windows: 可尝试 xterm / wt 等，这里简单返回说明
		http.Error(w, "multi-window connect only supported on macOS", http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// escapeSingleQuotes 用于 shell：' -> '\''
func escapeSingleQuotes(s string) string {
	var out string
	for _, c := range s {
		if c == '\'' {
			out += "'\\''"
		} else {
			out += string(c)
		}
	}
	return out
}

func escapeAppleScript(s string) string {
	out := ""
	for _, c := range s {
		if c == '\\' || c == '"' {
			out += "\\"
		}
		out += string(c)
	}
	return out
}
