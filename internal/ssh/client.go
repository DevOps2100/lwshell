package ssh

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"jumpserver-go/internal/models"
)

// ConnectOptions 连接时可覆盖的选项（如临时指定证书路径）
type ConnectOptions struct {
	KeyPathOverride string // 若不为空，则用此路径的私钥，忽略 Server.KeyPath
	WindowTitle     string // 若不为空，连接期间定期写入 /dev/tty 以固定窗口标题（防止远程覆盖）
}

// Connect 建立 SSH 连接并进入交互式终端；auth 优先使用证书（KeyPath），其次密码
func Connect(s models.Server, opts ConnectOptions) error {
	keyPath := s.KeyPath
	if opts.KeyPathOverride != "" {
		keyPath = opts.KeyPathOverride
	}

	config, err := buildClientConfig(s.User, s.Password, keyPath)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", s.Host, port(s))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	defer session.Close()

	// 请求 PTY 并启动 shell
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("标准输入不是终端，无法进入交互模式")
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, oldState)

	w, h, err := term.GetSize(fd)
	if err != nil {
		w, h = 80, 24
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", h, w, modes); err != nil {
		return fmt.Errorf("请求 PTY 失败: %w", err)
	}
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	// 窗口大小变化时通知服务端
	go watchWindowSize(session, fd)

	// 固定窗口标题：连接期间定期向 /dev/tty 写标题，避免远程覆盖状态栏
	if opts.WindowTitle != "" {
		done := make(chan struct{})
		defer close(done)
		go keepWindowTitle(done, opts.WindowTitle)
	}

	if err := session.Shell(); err != nil {
		return err
	}
	return session.Wait()
}

func port(s models.Server) int {
	if s.Port > 0 {
		return s.Port
	}
	return 22
}

func buildClientConfig(user, password, keyPath string) (*ssh.ClientConfig, error) {
	var auth []ssh.AuthMethod
	if keyPath != "" {
		keyAuth, err := readPrivateKey(keyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败: %w", err)
		}
		auth = append(auth, keyAuth)
	}
	if password != "" {
		auth = append(auth, ssh.Password(password))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("请配置密码或私钥路径")
	}
	return &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

func readPrivateKey(path string) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		if strings.Contains(err.Error(), "passphrase") {
			// 加密私钥：这里简化处理，可后续支持交互输入 passphrase
			return nil, fmt.Errorf("加密私钥暂不支持，请使用未加密私钥或配置密码登录: %w", err)
		}
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

// keepWindowTitle 定期向 /dev/tty 写入 OSC 标题，使状态栏始终显示服务器名（不被远程覆盖）
func keepWindowTitle(done <-chan struct{}, title string) {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer tty.Close()
	seq := "\033]0;" + title + "\007\033]2;" + title + "\007"
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		_, _ = tty.WriteString(seq)
		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
}

func watchWindowSize(session *ssh.Session, fd int) {
	// 简单实现：不阻塞主流程即可；完整实现可监听 SIGWINCH
	for {
		w, h, err := term.GetSize(fd)
		if err != nil {
			return
		}
		_ = session.WindowChange(h, w)
		// 可在此加 sleep 或 signal 监听，避免忙等
		return
	}
}

// PipeWindowSize 在后台循环检测终端尺寸变化并发送给 session（可选，用于完善体验）
func PipeWindowSize(session *ssh.Session, fd int, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			w, h, _ := term.GetSize(fd)
			_ = session.WindowChange(h, w)
		}
	}
}

// ReadAndSendStdin 将本地 stdin 转发到远程（通常由 session.Stdin 直接绑定 os.Stdin 完成）
func ReadAndSendStdin(dst io.Writer, src io.Reader) {
	io.Copy(dst, src)
}
