package models

import "time"

// OAuth2Client OAuth2客户端凭证
type OAuth2Client struct {
	ID               string    `json:"id"`
	ClientID         string    `json:"client_id"`
	ClientSecretHash string    `json:"-"`                             // 不返回密钥哈希
	ClientType       string    `json:"client_type"`                   // edge_node / third_party
	AllowedScopes    string    `json:"allowed_scopes"`                // 空格分隔的权限列表
	Description      string    `json:"description"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
