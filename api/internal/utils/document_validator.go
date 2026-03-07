package utils

import (
	"fmt"
	"io"
	"os"

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
