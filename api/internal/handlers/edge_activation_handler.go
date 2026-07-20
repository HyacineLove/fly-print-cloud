package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fly-print-cloud/api/internal/auth"
	"fly-print-cloud/api/internal/database"
	"fly-print-cloud/api/internal/security"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EdgeActivationHandler owns the two-stage device bootstrap. The activation
// secret is short lived and one time; long-lived OAuth credentials are never
// rendered in the Cloud UI or written to a downloadable file.
type EdgeActivationHandler struct {
	db      *database.DB
	clients *database.OAuth2ClientRepository
	cipher  *security.ClientSecretCipher
}

const edgeNodeRuntimeScopes = "edge:register edge:printer edge:heartbeat"

func NewEdgeActivationHandler(db *database.DB, clients *database.OAuth2ClientRepository, cipher *security.ClientSecretCipher) *EdgeActivationHandler {
	return &EdgeActivationHandler{db: db, clients: clients, cipher: cipher}
}

func activationCode() (string, error) {
	raw := make([]byte, 15)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	encoded := strings.TrimRight(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), "=")
	return encoded[:6] + "-" + encoded[6:12] + "-" + encoded[12:], nil
}

func hashActivationCode(code string) string {
	sum := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

// CreatePending creates an intentionally empty node record. Its only user
// visible value is the one-time code returned by this response.
func (h *EdgeActivationHandler) CreatePending(c *gin.Context) {
	code, err := activationCode()
	if err != nil {
		InternalErrorResponse(c, "生成激活码失败")
		return
	}
	nodeID := uuid.NewString()
	expiresAt := time.Now().Add(10 * time.Minute)
	_, err = h.db.Exec(`INSERT INTO edge_nodes(id,name,status,enabled,version,last_heartbeat,registration_state,activation_code_hash,activation_expires_at)
        VALUES($1,'','offline',true,'',CURRENT_TIMESTAMP,'pending_activation',$2,$3)`, nodeID, hashActivationCode(code), expiresAt)
	if err != nil {
		InternalErrorResponse(c, "创建待激活终端失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	CreatedResponse(c, gin.H{"id": nodeID, "activation_code": code, "expires_at": expiresAt, "registration_state": "pending_activation"})
}

type activateEdgeRequest struct {
	ActivationCode string `json:"activation_code" binding:"required"`
}

// Activate consumes an activation code atomically and creates the single
// device client bound to the pre-created node.
func (h *EdgeActivationHandler) Activate(c *gin.Context) {
	var req activateEdgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ValidationErrorResponse(c, err)
		return
	}
	tx, err := h.db.BeginTx()
	if err != nil {
		InternalErrorResponse(c, "激活服务暂不可用")
		return
	}
	defer tx.Rollback()
	var nodeID string
	err = tx.QueryRow(`SELECT id FROM edge_nodes WHERE registration_state='pending_activation' AND activation_code_hash=$1 AND activation_expires_at>CURRENT_TIMESTAMP AND deleted_at IS NULL FOR UPDATE`, hashActivationCode(req.ActivationCode)).Scan(&nodeID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "error": "invalid_activation_code", "message": "激活码无效或已过期"})
		return
	}
	rawSecret, err := auth.GenerateClientSecret()
	if err != nil {
		InternalErrorResponse(c, "生成设备凭据失败")
		return
	}
	secretHash, err := auth.HashClientSecret(rawSecret)
	if err != nil {
		InternalErrorResponse(c, "生成设备凭据失败")
		return
	}
	encrypted, err := h.cipher.Encrypt(rawSecret)
	if err != nil {
		InternalErrorResponse(c, "保护设备凭据失败")
		return
	}
	clientID := "edge-" + strings.ReplaceAll(nodeID, "-", "")
	var clientRecordID string
	err = tx.QueryRow(`INSERT INTO oauth2_clients(client_id,client_secret_hash,client_secret_encrypted,client_type,edge_node_id,allowed_scopes,description,enabled)
        VALUES($1,$2,$3,'edge_node',$4,$5,'自动签发的节点凭据',true) RETURNING id`, clientID, secretHash, encrypted, nodeID, edgeNodeRuntimeScopes).Scan(&clientRecordID)
	if err != nil {
		InternalErrorResponse(c, "创建节点凭据失败")
		return
	}
	result, err := tx.Exec(`UPDATE edge_nodes SET registration_state='registered', activated_at=CURRENT_TIMESTAMP, activation_code_hash=NULL, activation_expires_at=NULL WHERE id=$1 AND registration_state='pending_activation'`, nodeID)
	if err != nil {
		InternalErrorResponse(c, "激活终端失败")
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		InternalErrorResponse(c, "激活终端状态冲突")
		return
	}
	if err := tx.Commit(); err != nil {
		InternalErrorResponse(c, "提交终端激活失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	CreatedResponse(c, gin.H{"node_id": nodeID, "client_id": clientID, "client_secret": rawSecret, "auth_url": "/auth/token"})
}

// CleanupExpired removes only never-activated placeholder nodes.
func (h *EdgeActivationHandler) CleanupExpired() (int64, error) {
	result, err := h.db.Exec(`DELETE FROM edge_nodes WHERE registration_state='pending_activation' AND activation_expires_at<CURRENT_TIMESTAMP`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired pending nodes: %w", err)
	}
	return result.RowsAffected()
}

type edgeProfileRequest struct {
	Name             string `json:"name" binding:"required,min=1,max=100"`
	Location         string `json:"location"`
	Version          string `json:"version"`
	MACAddress       string `json:"mac_address"`
	OSVersion        string `json:"os_version"`
	CPUInfo          string `json:"cpu_info"`
	MemoryInfo       string `json:"memory_info"`
	DiskInfo         string `json:"disk_info"`
	NetworkInterface string `json:"network_interface"`
}

// UpdateSelfProfile accepts Edge-owned facts only after the newly activated
// device has obtained its node-bound OAuth token.
func (h *EdgeActivationHandler) UpdateSelfProfile(c *gin.Context) {
	var req edgeProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ValidationErrorResponse(c, err)
		return
	}
	nodeID := c.GetString("node_id")
	if nodeID == "" {
		UnauthorizedResponse(c, "节点身份无效")
		return
	}
	result, err := h.db.Exec(`UPDATE edge_nodes SET name=$2,location=$3,version=$4,mac_address=$5,os_version=$6,cpu_info=$7,memory_info=$8,disk_info=$9,network_interface=$10,registration_state='active' WHERE id=$1 AND enabled=true AND registration_state IN ('registered','active') AND deleted_at IS NULL`, nodeID, req.Name, req.Location, req.Version, req.MACAddress, req.OSVersion, req.CPUInfo, req.MemoryInfo, req.DiskInfo, req.NetworkInterface)
	if err != nil {
		InternalErrorResponse(c, "更新终端资料失败")
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "message": "节点不可用"})
		return
	}
	SuccessResponse(c, gin.H{"node_id": nodeID, "registration_state": "active"})
}
