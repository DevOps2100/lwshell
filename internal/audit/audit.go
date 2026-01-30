package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jumpserver-go/internal/models"
)

func logDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ssh-manager"), nil
}

func logPath() (string, error) {
	dir, err := logDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "access.log"), nil
}

// LogConnect 记录 SSH 连接尝试：主机、时间、成功与否（在 --connect-id 流程中调用）
func LogConnect(s *models.Server, connectErr error) {
	p, err := logPath()
	if err != nil {
		return
	}
	dir := filepath.Dir(p)
	_ = os.MkdirAll(dir, 0700)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().UTC().Format(time.RFC3339)
	port := s.Port
	if port <= 0 {
		port = 22
	}
	status := "success"
	if connectErr != nil {
		status = "failure"
	}
	line := fmt.Sprintf("%s connect id=%s name=%s host=%s port=%d user=%s %s",
		ts, s.ID, escape(s.Name), s.Host, port, escape(s.User), status)
	if connectErr != nil {
		line += fmt.Sprintf(" err=%s", escape(connectErr.Error()))
	}
	line += "\n"
	_, _ = f.WriteString(line)
}

func escape(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "\t", "_")
	if strings.ContainsAny(s, "\n\"\\") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
