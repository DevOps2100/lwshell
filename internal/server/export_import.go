package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lwshell/internal/config"
	"lwshell/internal/models"
)

// Export 导出完整服务器配置为 JSON（含密码），便于迁移或备份
func Export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="lwshell-servers.json"`)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

// ImportReq 导入请求：servers 为要导入的列表，replace 为 true 时替换全部，false 时与当前合并
type ImportReq struct {
	Servers []models.Server `json:"servers"`
	Replace bool            `json:"replace"`
}

// Import 导入 JSON 配置：replace 时替换全部，否则按 id 合并（存在则更新，不存在则追加）
func Import(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ImportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Servers == nil {
		req.Servers = []models.Server{}
	}
	cfg, err := config.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if req.Replace {
		cfg.Servers = req.Servers
	} else {
		existing := make(map[string]int)
		for i, s := range cfg.Servers {
			if s.ID != "" {
				existing[s.ID] = i
			}
		}
		maxID := 0
		for _, s := range cfg.Servers {
			if s.ID != "" {
				n, _ := strconv.Atoi(s.ID)
				if n > maxID {
					maxID = n
				}
			}
		}
		for _, s := range req.Servers {
			s.Name = strings.TrimSpace(s.Name)
			s.Host = strings.TrimSpace(s.Host)
			s.User = strings.TrimSpace(s.User)
			s.KeyPath = strings.TrimSpace(s.KeyPath)
			s.Group = strings.TrimSpace(s.Group)
			if s.Port <= 0 {
				s.Port = 22
			}
			if idx, ok := existing[s.ID]; ok && s.ID != "" {
				cfg.Servers[idx] = s
			} else {
				maxID++
				s.ID = strconv.Itoa(maxID)
				cfg.Servers = append(cfg.Servers, s)
			}
		}
	}
	if err := config.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(cfg.Servers),
	})
}
