package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"lwshell/internal/models"
)

// configPath 配置文件路径：os.UserConfigDir()/lwshell/servers.json
// macOS 为 ~/Library/Application Support/lwshell/servers.json，Linux 为 ~/.config/lwshell/servers.json
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "lwshell", "servers.json")
	return p, nil
}

// Load 从默认路径加载配置
func Load() (*models.Config, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &models.Config{Servers: []models.Server{}}, nil
		}
		return nil, err
	}
	var cfg models.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.Servers == nil {
		cfg.Servers = []models.Server{}
	}
	// 为没有 ID 的旧数据补全唯一 ID
	max := 0
	for _, s := range cfg.Servers {
		if s.ID != "" {
			n := parseNum(s.ID)
			if n > max {
				max = n
			}
		}
	}
	for i := range cfg.Servers {
		if cfg.Servers[i].ID == "" {
			max++
			cfg.Servers[i].ID = fmt.Sprintf("%d", max)
		}
	}
	return &cfg, nil
}

func parseNum(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// Save 保存配置到默认路径
func Save(cfg *models.Config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
