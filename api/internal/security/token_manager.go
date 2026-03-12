package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fly-print-cloud/api/internal/logger"

	"go.uber.org/zap"
)

const (
	// TokenTypeUpload 上传凭证类型
	TokenTypeUpload = "upload"
	// TokenTypeDownload 下载凭证类型
	TokenTypeDownload = "download"

	// DefaultTokenTTL 默认凭证有效期（秒）
	DefaultTokenTTL = 180 // 3分钟

	// DefaultSecret 默认密钥（仅用于开发环境）
	DefaultSecret = "fly-print-file-access-secret-dev-only"
)

// TokenUsageMarker 用于标记Token为已使用的接口
// 使用接口避免security包与database包的循环依赖
type TokenUsageMarker interface {
	// PreRegisterToken 预注册Token（在生成Token时调用）
	PreRegisterToken(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error

	// MarkTokenAsUsed 标记Token为已使用
	// 返回nil表示成功标记，返回错误表示token已被使用或其他错误
	MarkTokenAsUsed(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error
}

// TokenManager 凭证管理器
type TokenManager struct {
	secret           string
	uploadTokenTTL   int
	downloadTokenTTL int
	tokenRepo        TokenUsageMarker // 用于追踪token使用状态
}

// UploadTokenPayload 上传凭证载荷
type UploadTokenPayload struct {
	NodeID    string
	PrinterID string
	IssuedAt  int64
	ExpiresAt int64
}

// DownloadTokenPayload 下载凭证载荷
type DownloadTokenPayload struct {
	FileID    string
	JobID     string
	NodeID    string
	IssuedAt  int64
	ExpiresAt int64
}

// TokenError 凭证错误类型
type TokenError struct {
	Code    string
	Message string
}

func (e *TokenError) Error() string {
	return e.Message
}

// 预定义错误
var (
	ErrInvalidFormat    = &TokenError{Code: "invalid_format", Message: "Invalid token format"}
	ErrTokenExpired     = &TokenError{Code: "token_expired", Message: "Token has expired"}
	ErrInvalidSignature = &TokenError{Code: "invalid_signature", Message: "Invalid token signature"}
	ErrInvalidType      = &TokenError{Code: "invalid_type", Message: "Invalid token type"}
	ErrInvalidContext   = &TokenError{Code: "invalid_context", Message: "Token context mismatch"}
	ErrTokenAlreadyUsed = &TokenError{Code: "token_already_used", Message: "Token has already been used"}
)

// NewTokenManager 创建凭证管理器
// tokenRepo 用于追踪一次性token的使用状态，如果为nil则不启用一次性验证
func NewTokenManager(secret string, uploadTTL, downloadTTL int, tokenRepo TokenUsageMarker) *TokenManager {
	if secret == "" {
		secret = DefaultSecret
	}
	if uploadTTL <= 0 {
		uploadTTL = DefaultTokenTTL
	}
	if downloadTTL <= 0 {
		downloadTTL = DefaultTokenTTL
	}

	return &TokenManager{
		secret:           secret,
		uploadTokenTTL:   uploadTTL,
		downloadTokenTTL: downloadTTL,
		tokenRepo:        tokenRepo,
	}
}

// GenerateUploadToken 生成上传凭证
// 凭证格式: Base64(upload|nodeID|printerID|issuedAt|expiresAt|signature)
// 在生成新Token前，会先撤销该节点+打印机的所有旧Token，确保同一时刻只有一个有效Token
func (tm *TokenManager) GenerateUploadToken(nodeID, printerID string) (string, time.Time, error) {
	// 先撤销该节点+打印机的所有旧上传Token（防止Token堆积）
	if tm.tokenRepo != nil {
		// 使用类型断言获取完整的Repository接口
		if tokenRepo, ok := tm.tokenRepo.(interface {
			RevokeTokensByNodeAndResource(tokenType, nodeID, resourceID string) (int64, error)
		}); ok {
			revokedCount, err := tokenRepo.RevokeTokensByNodeAndResource(TokenTypeUpload, nodeID, printerID)
			if err != nil {
				logger.Warn("Failed to revoke old upload tokens for node and printer", zap.String("node_id", nodeID), zap.String("printer_id", printerID), zap.Error(err))
				// 不中断流程，继续生成新Token
			} else if revokedCount > 0 {
				logger.Debug("Revoked old upload tokens for node and printer", zap.Int64("count", revokedCount), zap.String("node_id", nodeID), zap.String("printer_id", printerID))
			}
		}
	}

	now := time.Now()
	issuedAt := now.Unix()
	expiresAt := now.Add(time.Duration(tm.uploadTokenTTL) * time.Second).Unix()

	// 构造 payload
	payload := fmt.Sprintf("%s|%s|%s|%d|%d", TokenTypeUpload, nodeID, printerID, issuedAt, expiresAt)

	// 生成签名
	signature := tm.generateSignature(payload)

	// 组合并 Base64 编码
	token := base64.StdEncoding.EncodeToString([]byte(payload + "|" + signature))

	// 预注册Token到数据库（确保撤销时可以找到）
	if tm.tokenRepo != nil {
		if preRegRepo, ok := tm.tokenRepo.(interface {
			PreRegisterToken(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error
		}); ok {
			tokenHash := generateTokenHash(token)
			err := preRegRepo.PreRegisterToken(tokenHash, TokenTypeUpload, nodeID, printerID, "", time.Unix(expiresAt, 0))
			if err != nil {
				logger.Warn("Failed to pre-register upload token", zap.Error(err))
				// 不阻断流程，继续返回token
			}
		}
	}

	return token, time.Unix(expiresAt, 0), nil
}

// GenerateDownloadToken 生成下载凭证
// 凭证格式: Base64(download|fileID|jobID|nodeID|issuedAt|expiresAt|signature)
// 在生成新Token前，会先撤销该节点+文件的所有旧Token，确保同一时刻只有一个有效Token
func (tm *TokenManager) GenerateDownloadToken(fileID, jobID, nodeID string) (string, time.Time, error) {
	// 先撤销该节点+文件的所有旧下载Token（防止Token堆积）
	if tm.tokenRepo != nil {
		// 使用类型断言获取完整的Repository接口
		if tokenRepo, ok := tm.tokenRepo.(interface {
			RevokeTokensByNodeAndResource(tokenType, nodeID, resourceID string) (int64, error)
		}); ok {
			revokedCount, err := tokenRepo.RevokeTokensByNodeAndResource(TokenTypeDownload, nodeID, fileID)
			if err != nil {
				logger.Warn("Failed to revoke old download tokens for node and file", zap.String("node_id", nodeID), zap.String("file_id", fileID), zap.Error(err))
				// 不中断流程，继续生成新Token
			} else if revokedCount > 0 {
				logger.Debug("Revoked old download tokens for node and file", zap.Int64("count", revokedCount), zap.String("node_id", nodeID), zap.String("file_id", fileID))
			}
		}
	}

	now := time.Now()
	issuedAt := now.Unix()
	expiresAt := now.Add(time.Duration(tm.downloadTokenTTL) * time.Second).Unix()

	// 构造 payload
	payload := fmt.Sprintf("%s|%s|%s|%s|%d|%d", TokenTypeDownload, fileID, jobID, nodeID, issuedAt, expiresAt)

	// 生成签名
	signature := tm.generateSignature(payload)

	// 组合并 Base64 编码
	token := base64.StdEncoding.EncodeToString([]byte(payload + "|" + signature))

	// 预注册Token到数据库（确保撤销时可以找到）
	if tm.tokenRepo != nil {
		if preRegRepo, ok := tm.tokenRepo.(interface {
			PreRegisterToken(tokenHash, tokenType, nodeID, resourceID, jobID string, expiresAt time.Time) error
		}); ok {
			tokenHash := generateTokenHash(token)
			err := preRegRepo.PreRegisterToken(tokenHash, TokenTypeDownload, nodeID, fileID, jobID, time.Unix(expiresAt, 0))
			if err != nil {
				logger.Warn("Failed to pre-register download token", zap.Error(err))
				// 不阻断流程，继续返回token
			}
		}
	}

	return token, time.Unix(expiresAt, 0), nil
}

// ValidateUploadToken 验证上传凭证（一次性凭证，使用后立即失效）
func (tm *TokenManager) ValidateUploadToken(token string) (*UploadTokenPayload, error) {
	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// 分割字段: upload|nodeID|printerID|issuedAt|expiresAt|signature
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 6 {
		return nil, ErrInvalidFormat
	}

	tokenType := parts[0]
	nodeID := parts[1]
	printerID := parts[2]
	issuedAtStr := parts[3]
	expiresAtStr := parts[4]
	signature := parts[5]

	// 验证类型
	if tokenType != TokenTypeUpload {
		return nil, ErrInvalidType
	}

	// 解析时间戳
	issuedAt, err := strconv.ParseInt(issuedAtStr, 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	expiresAt, err := strconv.ParseInt(expiresAtStr, 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// 验证时间
	now := time.Now().Unix()
	if now > expiresAt {
		return nil, ErrTokenExpired
	}

	// 重新计算签名并验证
	payload := fmt.Sprintf("%s|%s|%s|%d|%d", tokenType, nodeID, printerID, issuedAt, expiresAt)
	expectedSignature := tm.generateSignature(payload)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, ErrInvalidSignature
	}

	// 一次性Token验证：尝试标记为已使用
	if tm.tokenRepo != nil {
		tokenHash := generateTokenHash(token)
		err := tm.tokenRepo.MarkTokenAsUsed(
			tokenHash,
			TokenTypeUpload,
			nodeID,
			printerID, // resourceID为printerID
			"",        // 上传Token没有jobID
			time.Unix(expiresAt, 0),
		)
		if err != nil {
			// 检查是否是"已使用"错误
			if err.Error() == "token has already been used" {
				return nil, ErrTokenAlreadyUsed
			}
			// 检查是否是"已撤销"错误
			if err.Error() == "token has been revoked" {
				return nil, &TokenError{Code: "token_revoked", Message: "Token has been revoked"}
			}
			// 其他数据库错误，记录但仍允许通过（降级处理）
			// 这里可以选择返回错误或忽略，为了安全起见返回错误
			return nil, &TokenError{Code: "database_error", Message: "Failed to verify token usage"}
		}
	}

	return &UploadTokenPayload{
		NodeID:    nodeID,
		PrinterID: printerID,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateDownloadToken 验证下载凭证（一次性凭证，使用后立即失效）
func (tm *TokenManager) ValidateDownloadToken(token, expectedFileID, expectedNodeID string) (*DownloadTokenPayload, error) {
	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// 分割字段: download|fileID|jobID|nodeID|issuedAt|expiresAt|signature
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 7 {
		return nil, ErrInvalidFormat
	}

	tokenType := parts[0]
	fileID := parts[1]
	jobID := parts[2]
	nodeID := parts[3]
	issuedAtStr := parts[4]
	expiresAtStr := parts[5]
	signature := parts[6]

	// 验证类型
	if tokenType != TokenTypeDownload {
		return nil, ErrInvalidType
	}

	// 解析时间戳
	issuedAt, err := strconv.ParseInt(issuedAtStr, 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	expiresAt, err := strconv.ParseInt(expiresAtStr, 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// 验证时间
	now := time.Now().Unix()
	if now > expiresAt {
		return nil, ErrTokenExpired
	}

	// 重新计算签名并验证
	payload := fmt.Sprintf("%s|%s|%s|%s|%d|%d", tokenType, fileID, jobID, nodeID, issuedAt, expiresAt)
	expectedSignature := tm.generateSignature(payload)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, ErrInvalidSignature
	}

	// 验证上下文（如果提供了预期值）
	if expectedFileID != "" && fileID != expectedFileID {
		return nil, ErrInvalidContext
	}
	if expectedNodeID != "" && nodeID != expectedNodeID {
		return nil, ErrInvalidContext
	}

	// 一次性Token验证：尝试标记为已使用
	if tm.tokenRepo != nil {
		tokenHash := generateTokenHash(token)
		err := tm.tokenRepo.MarkTokenAsUsed(
			tokenHash,
			TokenTypeDownload,
			nodeID,
			fileID, // resourceID为fileID
			jobID,
			time.Unix(expiresAt, 0),
		)
		if err != nil {
			// 检查是否是"已使用"错误
			if err.Error() == "token has already been used" {
				return nil, ErrTokenAlreadyUsed
			}
			// 检查是否是"已撤销"错误
			if err.Error() == "token has been revoked" {
				return nil, &TokenError{Code: "token_revoked", Message: "Token has been revoked"}
			}
			// 其他数据库错误，为了安全起见返回错误
			return nil, &TokenError{Code: "database_error", Message: "Failed to verify token usage"}
		}
	}

	return &DownloadTokenPayload{
		FileID:    fileID,
		JobID:     jobID,
		NodeID:    nodeID,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateDownloadTokenSimple 简单验证下载凭证（不检查上下文）
func (tm *TokenManager) ValidateDownloadTokenSimple(token string) (*DownloadTokenPayload, error) {
	return tm.ValidateDownloadToken(token, "", "")
}

// VerifyUploadTokenLightweight 轻量验证上传Token（仅验证格式、签名、过期时间，不标记为已使用）
// 用于页面加载时的初步验证，避免消耗一次性Token
func (tm *TokenManager) VerifyUploadTokenLightweight(token string) (*UploadTokenPayload, error) {
	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// 分割字段: upload|nodeID|printerID|issuedAt|expiresAt|signature
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 6 {
		return nil, ErrInvalidFormat
	}

	tokenType := parts[0]
	nodeID := parts[1]
	printerID := parts[2]
	issuedAtStr := parts[3]
	expiresAtStr := parts[4]
	signature := parts[5]

	// 验证类型
	if tokenType != TokenTypeUpload {
		return nil, ErrInvalidType
	}

	// 解析时间戳
	issuedAt, err := strconv.ParseInt(issuedAtStr, 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	expiresAt, err := strconv.ParseInt(expiresAtStr, 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// 验证时间
	now := time.Now().Unix()
	if now > expiresAt {
		return nil, ErrTokenExpired
	}

	// 重新计算签名并验证
	payload := fmt.Sprintf("%s|%s|%s|%d|%d", tokenType, nodeID, printerID, issuedAt, expiresAt)
	expectedSignature := tm.generateSignature(payload)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, ErrInvalidSignature
	}

	// 注意：此方法不标记Token为已使用，仅用于轻量验证
	return &UploadTokenPayload{
		NodeID:    nodeID,
		PrinterID: printerID,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}, nil
}

// generateSignature 生成 HMAC-SHA256 签名
func (tm *TokenManager) generateSignature(payload string) string {
	h := hmac.New(sha256.New, []byte(tm.secret))
	h.Write([]byte(payload))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// generateTokenHash 生成Token的SHA256哈希值
// 用于在数据库中唯一标识Token，避免存储完整Token
func generateTokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}

// IsTokenError 检查错误是否为凭证错误
func IsTokenError(err error) bool {
	var tokenErr *TokenError
	return errors.As(err, &tokenErr)
}

// GetTokenErrorCode 获取凭证错误码
func GetTokenErrorCode(err error) string {
	var tokenErr *TokenError
	if errors.As(err, &tokenErr) {
		return tokenErr.Code
	}
	return "unknown_error"
}
