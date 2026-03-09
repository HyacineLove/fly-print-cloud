package utils

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// ValidatePDFPageCount 验证 PDF 文件页数
// 参数:
//   - filePath: PDF 文件路径
//   - maxPages: 允许的最大页数
//
// 返回:
//   - pageCount: 实际页数
//   - error: 如果页数超限或读取失败则返回错误
func ValidatePDFPageCount(filePath string, maxPages int) (int, error) {
	// 打开 PDF 文件
	f, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer f.Close()

	// 使用 pdfcpu 获取页数
	pageCount, err := api.PageCountFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to count PDF pages: %w", err)
	}

	// 检查页数是否超限
	if pageCount > maxPages {
		return pageCount, fmt.Errorf("PDF has %d pages, exceeds maximum of %d pages", pageCount, maxPages)
	}

	return pageCount, nil
}

// ValidatePDFPageCountFromReader 从 io.Reader 验证 PDF 页数（需要 io.ReadSeeker）
// 参数:
//   - reader: PDF 文件读取器
//   - maxPages: 允许的最大页数
//
// 返回:
//   - pageCount: 实际页数
//   - error: 如果页数超限或读取失败则返回错误
func ValidatePDFPageCountFromReader(reader io.ReadSeeker, maxPages int) (int, error) {
	// 使用 pdfcpu 读取页数
	pageCount, err := api.PageCount(reader, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count PDF pages: %w", err)
	}

	// 检查页数是否超限
	if pageCount > maxPages {
		return pageCount, fmt.Errorf("PDF has %d pages, exceeds maximum of %d pages", pageCount, maxPages)
	}

	return pageCount, nil
}

type docxAppProperties struct {
	Pages string `xml:"Pages"`
}

func ValidateDocumentPageCountFromReader(file io.ReaderAt, seeker io.ReadSeeker, fileSize int64, ext string, maxPages int) (int, error) {
	normalizedExt := strings.ToLower(ext)
	var pageCount int
	var err error
	switch normalizedExt {
	case ".pdf":
		pageCount, err = ValidatePDFPageCountFromReader(seeker, maxPages)
	case ".docx":
		pageCount, err = getDOCXPageCount(file, fileSize)
	case ".doc":
		pageCount, err = getDOCPageCount(seeker)
	default:
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if pageCount > maxPages {
		return pageCount, fmt.Errorf("document has %d pages, exceeds maximum of %d pages", pageCount, maxPages)
	}
	return pageCount, nil
}

func getDOCXPageCount(file io.ReaderAt, fileSize int64) (int, error) {
	reader, err := zip.NewReader(file, fileSize)
	if err != nil {
		return 0, fmt.Errorf("failed to read docx archive: %w", err)
	}
	for _, entry := range reader.File {
		if entry.Name != "docProps/app.xml" {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return 0, fmt.Errorf("failed to open docx metadata: %w", err)
		}
		content, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return 0, fmt.Errorf("failed to read docx metadata: %w", readErr)
		}
		if closeErr != nil {
			return 0, fmt.Errorf("failed to close docx metadata: %w", closeErr)
		}
		var app docxAppProperties
		if err := xml.Unmarshal(content, &app); err != nil {
			return 0, fmt.Errorf("failed to parse docx metadata: %w", err)
		}
		pages, err := strconv.Atoi(strings.TrimSpace(app.Pages))
		if err != nil || pages <= 0 {
			return 0, fmt.Errorf("invalid docx page metadata")
		}
		return pages, nil
	}
	return 0, fmt.Errorf("docx page metadata not found")
}

func getDOCPageCount(seeker io.ReadSeeker) (int, error) {
	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek doc file: %w", err)
	}
	content, err := io.ReadAll(seeker)
	if err != nil {
		return 0, fmt.Errorf("failed to read doc file: %w", err)
	}
	if len(content) == 0 {
		return 0, fmt.Errorf("empty doc file")
	}
	return bytes.Count(content, []byte{0x0c}) + 1, nil
}
