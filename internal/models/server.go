package models

// Server 表示一台 SSH 主机配置
type Server struct {
	ID       string `json:"id"`       // 唯一标识
	Name     string `json:"name"`     // 显示名称
	Host     string `json:"host"`     // IP 或域名
	Port     int    `json:"port"`     // 端口，默认 22
	User     string `json:"user"`     // 登录用户
	Password string `json:"password"` // 密码（可选，与证书二选一或都填）
	KeyPath  string `json:"key_path"` // 私钥/证书路径（可选）
	Group    string `json:"group"`    // 分组名称，用于分组显示
}

// Config 持久化配置：服务器列表
type Config struct {
	Servers []Server `json:"servers"`
}
