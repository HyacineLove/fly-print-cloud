package database

import (
	"errors"
	"time"
)

// ErrTokenAlreadyUsed Token已被使用错误
var ErrTokenAlreadyUsed = errors.New("token has already been used")

// TokenUsageRecord Token使用记录
type TokenUsageRecord struct {
	TokenHash  string    `json:"token_hash"`
	TokenType  string    `json:"token_type"`
	NodeID     string    `json:"node_id"`
	ResourceID string    `json:"resource_id"`
	JobID      string    `json:"job_id"`
	UsedAt     time.Time `json:"used_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// TokenUsageRepository Token使用记录仓库
type TokenUsageRepository struct {
	db *DB
}

// NewTokenUsageRepository 创建Token使用记录仓库
func NewTokenUsageRepository(db *DB) *TokenUsageRepository {
	return &TokenUsageRepository{db: db}
}

// MarkTokenAsUsed 标记Token为已使用（原子操作）
// 使用INSERT ... ON CONFLICT DO NOTHING实现原子性检测
// 返回nil表示成功标记，返回ErrTokenAlreadyUsed表示token已被使用
func (r *TokenUsageRepository) MarkTokenAsUsed(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error {
	// 使用INSERT ... ON CONFLICT DO NOTHING
	// 如果token_hash已存在（主键冲突），不执行任何操作
	// 通过检查影响的行数来判断是首次使用还是重复使用
	query := `
		INSERT INTO token_usage_records (token_hash, token_type, node_id, resource_id, job_id, expires_at, used_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (token_hash) DO NOTHING
	`

	result, err := r.db.Exec(query, tokenHash, tokenType, nodeID, resourceID, jobID, expiresAt)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// 如果没有插入任何行，说明token已经被使用过
	if rowsAffected == 0 {
		return ErrTokenAlreadyUsed
	}

	return nil
}

// IsTokenUsed 检查Token是否已被使用
func (r *TokenUsageRepository) IsTokenUsed(tokenHash string) (bool, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM token_usage_records WHERE token_hash = $1", tokenHash).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CleanupExpiredTokens 清理过期的Token记录
// 删除expires_at早于指定时间的记录
// 返回删除的记录数
func (r *TokenUsageRepository) CleanupExpiredTokens(before time.Time) (int64, error) {
	result, err := r.db.Exec("DELETE FROM token_usage_records WHERE expires_at < $1", before)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// RevokeTokensByNodeAndResource 撤销指定节点和资源的所有旧Token
// 用于在生成新Token时，使旧Token失效（通过删除记录实现）
// tokenType: "upload" 或 "download"
// nodeID: 节点ID
// resourceID: 资源ID（上传Token为printerID，下载Token为fileID）
// 返回撤销的Token数量
func (r *TokenUsageRepository) RevokeTokensByNodeAndResource(tokenType, nodeID, resourceID string) (int64, error) {
	query := `
		DELETE FROM token_usage_records 
		WHERE token_type = $1 
		  AND node_id = $2 
		  AND resource_id = $3
	`
	
	result, err := r.db.Exec(query, tokenType, nodeID, resourceID)
	if err != nil {
		return 0, err
	}
	
	return result.RowsAffected()
}

// RevokeAllTokensByNode 撤销指定节点的所有Token（所有类型、所有资源）
// 用于节点下线或重连时清理所有旧Token
func (r *TokenUsageRepository) RevokeAllTokensByNode(nodeID string) (int64, error) {
	result, err := r.db.Exec("DELETE FROM token_usage_records WHERE node_id = $1", nodeID)
	if err != nil {
		return 0, err
	}
	
	return result.RowsAffected()
}

// RevokeTokensByNodeAndType 撤销指定节点的指定类型Token（例如仅上传Token）
// tokenType: "upload" 或 "download"
// nodeID: 节点ID
func (r *TokenUsageRepository) RevokeTokensByNodeAndType(tokenType, nodeID string) (int64, error) {
	result, err := r.db.Exec("DELETE FROM token_usage_records WHERE token_type = $1 AND node_id = $2", tokenType, nodeID)
	if err != nil {
		return 0, err
	}
	
	return result.RowsAffected()
}

// GetTokenUsageStats 获取Token使用统计（用于监控）
func (r *TokenUsageRepository) GetTokenUsageStats() (map[string]int64, error) {
	stats := make(map[string]int64)

	// 获取各类型Token的使用数量
	rows, err := r.db.Query("SELECT token_type, COUNT(*) FROM token_usage_records GROUP BY token_type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tokenType string
		var count int64
		if err := rows.Scan(&tokenType, &count); err != nil {
			return nil, err
		}
		stats[tokenType] = count
	}

	// 获取总数
	var total int64
	err = r.db.QueryRow("SELECT COUNT(*) FROM token_usage_records").Scan(&total)
	if err != nil {
		return nil, err
	}
	stats["total"] = total

	return stats, nil
}
