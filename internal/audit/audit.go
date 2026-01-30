package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lwshell/internal/models"
)

func logDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lwshell"), nil
}

func logPath() (string, error) {
	dir, err := logDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "access.log"), nil
}

// writeLogLine 向 access.log 写入一行并立即 Sync，确保进程异常退出时也能落盘
func writeLogLine(line string) {
	p, err := logPath()
	if err != nil {
		return
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Sync()
	_ = f.Close()
}

// LogConnectStart 在发起 SSH 连接时立即记录（点击「连接」后、ssh.Connect 阻塞前调用）
func LogConnectStart(s *models.Server) {
	ts := time.Now().UTC().Format(time.RFC3339)
	port := s.Port
	if port <= 0 {
		port = 22
	}
	line := fmt.Sprintf("%s connect id=%s name=%s host=%s port=%d user=%s status=started\n",
		ts, s.ID, escape(s.Name), s.Host, port, escape(s.User))
	writeLogLine(line)
}

// LogConnect 记录 SSH 连接结束：成功或失败（在 ssh.Connect 返回后调用）
func LogConnect(s *models.Server, connectErr error) {
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
	writeLogLine(line)
}

func escape(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "\t", "_")
	if strings.ContainsAny(s, "\n\"\\") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
