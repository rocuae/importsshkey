package domain

// Source 数据源配置，定义一个公钥拉取源的完整信息
type Source struct {
	// Alias 短别名，用于命令行快速引用
	Alias string
	// URL 静态 URL 或模板 URL，支持 {{ .VarName }} 语法
	URL string
	// Format 返回内容解析格式: plaintext 或 github_json
	Format string
	// AuthRef 引用的凭证名称
	AuthRef string
	// DefaultVars 默认模板变量值
	DefaultVars map[string]string
}
