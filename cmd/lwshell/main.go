package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"lwshell/internal/audit"
	"lwshell/internal/auth"
	"lwshell/internal/config"
	"lwshell/internal/models"
	"lwshell/internal/server"
	"lwshell/internal/ssh"
)

//go:embed web
var webFS embed.FS

const defaultHTTPAddr = ":21008"

func main() {
	connectID := flag.String("connect-id", "", "直接连接指定 ID 的服务器（供 Web 在新终端调用）")
	httpAddr := flag.String("http", defaultHTTPAddr, "启动 Web 服务地址，例如 :21008")
	flag.Parse()

	if *connectID != "" {
		runConnect(*connectID)
		return
	}
	runHTTP(*httpAddr)
}

func runConnect(id string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var target *models.Server
	for i := range cfg.Servers {
		if cfg.Servers[i].ID == id {
			target = &cfg.Servers[i]
			break
		}
	}
	if target == nil {
		fmt.Fprintln(os.Stderr, "server not found:", id)
		os.Exit(1)
	}
	// 立即写入「开始连接」日志，避免用户直接关终端时没有记录
	audit.LogConnectStart(target)
	// 在终端中显示当前连接的服务器，并固定窗口标题为服务器名（连接期间会定期刷新，防止被远程覆盖）
	showServerBanner(target)
	port := target.Port
	if port <= 0 {
		port = 22
	}
	title := fmt.Sprintf("SSH: %s (%s@%s:%d)", target.Name, target.User, target.Host, port)
	connectErr := ssh.Connect(*target, ssh.ConnectOptions{WindowTitle: title})
	audit.LogConnect(target, connectErr)
	if connectErr != nil {
		fmt.Fprintln(os.Stderr, connectErr)
		os.Exit(1)
	}
}

// showServerBanner 在终端打印服务器标识，并设置 Terminal 窗口/标签标题
func showServerBanner(s *models.Server) {
	port := s.Port
	if port <= 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s@%s:%d", s.User, s.Host, port)
	title := fmt.Sprintf("SSH: %s (%s)", s.Name, addr)
	banner := fmt.Sprintf("\n  ═══ %s ═══\n  主机: %s  |  用户: %s  |  端口: %d\n  %s\n\n",
		s.Name, s.Host, s.User, port, addr)
	// 设置窗口标题（macOS Terminal / iTerm 等支持 OSC 0 和 OSC 2）
	fmt.Print("\033]0;", title, "\007")
	fmt.Print("\033]2;", title, "\007")
	fmt.Print(banner)
}

func runHTTP(addr string) {
	// 启动前先关闭占用目标端口的进程（避免重复启动需手动关旧服务）
	if port := getListenPort(addr); port != "" {
		killProcessOnPort(port)
		time.Sleep(800 * time.Millisecond)
	}

	mux := http.NewServeMux()
	// 认证：状态、首次设置密码、登录、登出（无需登录）
	mux.HandleFunc("/api/auth/status", auth.Status)
	mux.HandleFunc("/api/auth/setup", auth.Setup)
	mux.HandleFunc("/api/auth/login", auth.Login)
	mux.HandleFunc("/api/auth/logout", auth.Logout)
	mux.HandleFunc("/api/auth/reset", auth.RequireAuth(auth.Reset))
	// 以下接口需登录
	mux.HandleFunc("/api/servers", auth.RequireAuth(server.ServersAPI))
	mux.HandleFunc("/api/servers/", auth.RequireAuth(server.ServersAPI))
	mux.HandleFunc("/api/connect", auth.RequireAuth(server.Connect))
	mux.HandleFunc("/api/export", auth.RequireAuth(server.Export))
	mux.HandleFunc("/api/import", auth.RequireAuth(server.Import))
	webRoot, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(webRoot)))
	fmt.Println("lwshell Web: http://127.0.0.1" + addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getListenPort 从监听地址解析端口，如 ":21008" -> "21008"
func getListenPort(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") && addr[1:] != "" {
			return strings.TrimPrefix(addr, ":")
		}
		return ""
	}
	if port != "" {
		return port
	}
	_ = host
	return ""
}

// killProcessOnPort 关闭占用指定端口的进程（macOS/Linux 使用 lsof + kill）
func killProcessOnPort(port string) {
	if runtime.GOOS == "windows" {
		return
	}
	cmd := exec.Command("lsof", "-i", ":"+port, "-t")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		proc, _ := os.FindProcess(pid)
		if proc != nil {
			_ = proc.Kill()
		}
	}
}
