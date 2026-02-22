package models

import (
	"time"
)

// File 文件模型
type File struct {
	ID           string    `json:"id" db:"id"`
	OriginalName string    `json:"original_name" db:"original_name"`
	FileName     string    `json:"-" db:"file_name"` // 存储在磁盘上的文件名
	FilePath     string    `json:"-" db:"file_path"` // 相对存储根目录的路径
	MimeType     string    `json:"mime_type" db:"mime_type"`
	Size         int64     `json:"size" db:"size"`
	UploaderID   string    `json:"uploader_id" db:"uploader_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	URL          string    `json:"url" db:"-"` // 动态生成
}
