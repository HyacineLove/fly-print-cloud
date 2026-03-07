package database

import (
	"database/sql"
	"fmt"
	"time"

	"fly-print-cloud/api/internal/models"

	"github.com/google/uuid"
)

type PrintJobRepository struct {
	db *DB
}

func NewPrintJobRepository(db *DB) *PrintJobRepository {
	return &PrintJobRepository{db: db}
}

// CreatePrintJob 创建打印任务
func (r *PrintJobRepository) CreatePrintJob(job *models.PrintJob) error {
	query := `
		INSERT INTO print_jobs (
			id, name, status, printer_id, 
			user_id, user_name, file_path, file_url, file_size, page_count, 
			copies, paper_size, color_mode, duplex_mode, 
			start_time, end_time, error_message, retry_count, 
			max_retries, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 
			$12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		)`

	now := time.Now()
	job.ID = uuid.New().String()
	job.CreatedAt = now
	job.UpdatedAt = now

	// 将user_id转换为sql.NullString以支持可空值
	var userID sql.NullString
	if job.UserID != "" {
		userID = sql.NullString{String: job.UserID, Valid: true}
	}

	_, err := r.db.DB.Exec(query,
		job.ID, job.Name, job.Status, job.PrinterID,
		userID, job.UserName, job.FilePath, job.FileURL, job.FileSize, job.PageCount,
		job.Copies, job.PaperSize, job.ColorMode, job.DuplexMode,
		job.StartTime, job.EndTime, job.ErrorMessage, job.RetryCount,
		job.MaxRetries, job.CreatedAt, job.UpdatedAt,
	)

	return err
}

// GetPrintJobByID 根据ID获取打印任务
func (r *PrintJobRepository) GetPrintJobByID(id string) (*models.PrintJob, error) {
	query := `
		SELECT id, name, status, printer_id, 
			   user_id, user_name, file_path, file_url, file_size, page_count, 
			   copies, paper_size, color_mode, duplex_mode, 
			   start_time, end_time, error_message, retry_count, 
			   max_retries, created_at, updated_at
		FROM print_jobs WHERE id = $1`

	job := &models.PrintJob{}
	var userID sql.NullString
	err := r.db.DB.QueryRow(query, id).Scan(
		&job.ID, &job.Name, &job.Status, &job.PrinterID,
		&userID, &job.UserName, &job.FilePath, &job.FileURL, &job.FileSize, &job.PageCount,
		&job.Copies, &job.PaperSize, &job.ColorMode, &job.DuplexMode,
		&job.StartTime, &job.EndTime, &job.ErrorMessage, &job.RetryCount,
		&job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
	)

	// 有值就设置，没值就空着
	if userID.Valid {
		job.UserID = userID.String
	}

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return job, nil
}

// ListPrintJobs 获取打印任务列表
func (r *PrintJobRepository) ListPrintJobs(limit, offset int, status, printerID, userID, edgeNodeID string, startTime, endTime *time.Time) ([]*models.PrintJob, error) {
	query := `
		SELECT pj.id, pj.name, pj.status, pj.printer_id, 
			   pj.user_id, pj.user_name, pj.file_path, pj.file_url, pj.file_size, pj.page_count, 
			   pj.copies, pj.paper_size, pj.color_mode, pj.duplex_mode, 
			   pj.start_time, pj.end_time, pj.error_message, pj.retry_count, 
			   pj.max_retries, pj.created_at, pj.updated_at,
			   COALESCE(p.edge_node_id, '') as edge_node_id
		FROM print_jobs pj
		LEFT JOIN printers p ON pj.printer_id = p.id
		WHERE 1=1`

	args := []interface{}{}
	argIndex := 1

	if status != "" {
		query += fmt.Sprintf(" AND pj.status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	if printerID != "" {
		query += fmt.Sprintf(" AND pj.printer_id = $%d", argIndex)
		args = append(args, printerID)
		argIndex++
	}

	if userID != "" {
		query += fmt.Sprintf(" AND pj.user_id = $%d", argIndex)
		args = append(args, userID)
		argIndex++
	}

	if edgeNodeID != "" {
		query += fmt.Sprintf(" AND p.edge_node_id = $%d", argIndex)
		args = append(args, edgeNodeID)
		argIndex++
	}

	if startTime != nil {
		query += fmt.Sprintf(" AND pj.created_at >= $%d", argIndex)
		args = append(args, *startTime)
		argIndex++
	}

	if endTime != nil {
		query += fmt.Sprintf(" AND pj.created_at < $%d", argIndex)
		args = append(args, *endTime)
		argIndex++
	}

	query += " ORDER BY pj.created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
		argIndex++
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	rows, err := r.db.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.PrintJob
	for rows.Next() {
		job := &models.PrintJob{}
		var userID sql.NullString
		var edgeNodeID sql.NullString
		err := rows.Scan(
			&job.ID, &job.Name, &job.Status, &job.PrinterID,
			&userID, &job.UserName, &job.FilePath, &job.FileURL, &job.FileSize, &job.PageCount,
			&job.Copies, &job.PaperSize, &job.ColorMode, &job.DuplexMode,
			&job.StartTime, &job.EndTime, &job.ErrorMessage, &job.RetryCount,
			&job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
			&edgeNodeID,
		)
		if err != nil {
			return nil, err
		}

		// 有值就设置，没值就空着
		if userID.Valid {
			job.UserID = userID.String
		}
		if edgeNodeID.Valid {
			job.EdgeNodeID = edgeNodeID.String
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// UpdatePrintJob 更新打印任务
func (r *PrintJobRepository) UpdatePrintJob(job *models.PrintJob) error {
	query := `
		UPDATE print_jobs SET 
			name = $2, status = $3, file_path = $4, 
			file_size = $5, page_count = $6, copies = $7, paper_size = $8, 
			color_mode = $9, duplex_mode = $10, start_time = $11, 
			end_time = $12, error_message = $13, retry_count = $14, 
			max_retries = $15, updated_at = $16
		WHERE id = $1`

	job.UpdatedAt = time.Now()

	_, err := r.db.DB.Exec(query,
		job.ID, job.Name, job.Status, job.FilePath,
		job.FileSize, job.PageCount, job.Copies, job.PaperSize,
		job.ColorMode, job.DuplexMode, job.StartTime,
		job.EndTime, job.ErrorMessage, job.RetryCount,
		job.MaxRetries, job.UpdatedAt,
	)

	return err
}

// DeletePrintJob 删除打印任务
func (r *PrintJobRepository) DeletePrintJob(id string) error {
	query := `DELETE FROM print_jobs WHERE id = $1`
	_, err := r.db.DB.Exec(query, id)
	return err
}

// GetPrintJobsByPrinterID 根据打印机ID获取任务列表
func (r *PrintJobRepository) GetPrintJobsByPrinterID(printerID string, limit, offset int) ([]*models.PrintJob, error) {
	return r.ListPrintJobs(limit, offset, "", printerID, "", "", nil, nil)
}

// GetPrintJobsByUserID 根据用户ID获取任务列表
func (r *PrintJobRepository) GetPrintJobsByUserID(userID string, limit, offset int) ([]*models.PrintJob, error) {
	return r.ListPrintJobs(limit, offset, "", "", userID, "", nil, nil)
}

// GetPendingOrDispatchedJobsByEdgeNodeID 获取指定节点下所有待处理或已分发但未完成的任务
func (r *PrintJobRepository) GetPendingOrDispatchedJobsByEdgeNodeID(edgeNodeID string) ([]*models.PrintJob, error) {
	query := `
		SELECT pj.id, pj.name, pj.status, pj.printer_id, p.name,
			   pj.user_id, pj.user_name, pj.file_path, pj.file_url, pj.file_size, pj.page_count, 
			   pj.copies, pj.paper_size, pj.color_mode, pj.duplex_mode, 
			   pj.start_time, pj.end_time, pj.error_message, pj.retry_count, 
			   pj.max_retries, pj.created_at, pj.updated_at
		FROM print_jobs pj
		JOIN printers p ON pj.printer_id = p.id
		WHERE p.edge_node_id = $1 
		AND pj.status IN ('pending', 'dispatched')
		ORDER BY pj.created_at ASC`

	rows, err := r.db.DB.Query(query, edgeNodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.PrintJob
	for rows.Next() {
		job := &models.PrintJob{}
		var userID sql.NullString
		err := rows.Scan(
			&job.ID, &job.Name, &job.Status, &job.PrinterID, &job.PrinterName,
			&userID, &job.UserName, &job.FilePath, &job.FileURL, &job.FileSize, &job.PageCount,
			&job.Copies, &job.PaperSize, &job.ColorMode, &job.DuplexMode,
			&job.StartTime, &job.EndTime, &job.ErrorMessage, &job.RetryCount,
			&job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if userID.Valid {
			job.UserID = userID.String
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// UpdateJobStatus 更新打印任务状态和进度
func (r *PrintJobRepository) UpdateJobStatus(jobID, status string, progress int, errorMessage string) error {
	query := `
		UPDATE print_jobs SET 
			status = $2, 
			error_message = $3,
			updated_at = $4,
			start_time = CASE WHEN $5 = 'printing' THEN $4 ELSE start_time END,
			end_time = CASE WHEN $6 IN ('completed', 'failed', 'cancelled') THEN $4 ELSE end_time END
		WHERE id = $1`

	now := time.Now()
	_, err := r.db.DB.Exec(query, jobID, status, errorMessage, now, status, status)
	return err
}

// CountPrintJobs 统计打印任务总数
func (r *PrintJobRepository) CountPrintJobs(status, printerID, userID, edgeNodeID string, startTime, endTime *time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM print_jobs pj`

	// 如果需要按节点ID筛选，需要 JOIN printers 表
	if edgeNodeID != "" {
		query += ` LEFT JOIN printers p ON pj.printer_id = p.id`
	}

	query += ` WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	if status != "" {
		query += fmt.Sprintf(" AND pj.status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}

	if printerID != "" {
		query += fmt.Sprintf(" AND pj.printer_id = $%d", argIndex)
		args = append(args, printerID)
		argIndex++
	}

	if userID != "" {
		query += fmt.Sprintf(" AND pj.user_id = $%d", argIndex)
		args = append(args, userID)
		argIndex++
	}

	if edgeNodeID != "" {
		query += fmt.Sprintf(" AND p.edge_node_id = $%d", argIndex)
		args = append(args, edgeNodeID)
		argIndex++
	}

	if startTime != nil {
		query += fmt.Sprintf(" AND pj.created_at >= $%d", argIndex)
		args = append(args, *startTime)
		argIndex++
	}

	if endTime != nil {
		query += fmt.Sprintf(" AND pj.created_at < $%d", argIndex)
		args = append(args, *endTime)
		argIndex++
	}

	var total int
	err := r.db.DB.QueryRow(query, args...).Scan(&total)
	return total, err
}

// CountJobsByStatusAndDate 根据状态和日期范围统计打印任务数量
func (r *PrintJobRepository) CountJobsByStatusAndDate(status string, startDate, endDate time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM print_jobs WHERE status = $1 AND created_at >= $2 AND created_at < $3`

	var count int
	err := r.db.DB.QueryRow(query, status, startDate, endDate).Scan(&count)
	return count, err
}

// CountActiveJobsByPrinter 统计指定打印机的活动任务数（pending, dispatched, printing 状态）
func (r *PrintJobRepository) CountActiveJobsByPrinter(printerID string) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM print_jobs 
		WHERE printer_id = $1 AND status IN ('pending', 'dispatched', 'printing')`

	var count int
	err := r.db.DB.QueryRow(query, printerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active jobs by printer: %w", err)
	}
	return count, nil
}

// CleanupStaleJobs 标记长时间未更新的“打印中”任务为失败
func (r *PrintJobRepository) CleanupStaleJobs(timeout time.Duration) (int64, error) {
	query := `
		UPDATE print_jobs 
		SET status = 'failed', 
			error_message = 'Job timed out - Edge node did not report status',
			end_time = $1,
			updated_at = $1
		WHERE status IN ('pending', 'dispatched', 'printing') 
		AND updated_at < $2
	`

	now := time.Now()
	threshold := now.Add(-timeout)

	result, err := r.db.DB.Exec(query, now, threshold)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

// ListPrintJobsWithTotal 获取打印任务列表并返回总数
func (r *PrintJobRepository) ListPrintJobsWithTotal(limit, offset int, status, printerID, userID, edgeNodeID string, startTime, endTime *time.Time) ([]*models.PrintJob, int, error) {
	// 获取总数
	total, err := r.CountPrintJobs(status, printerID, userID, edgeNodeID, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}

	// 获取列表
	jobs, err := r.ListPrintJobs(limit, offset, status, printerID, userID, edgeNodeID, startTime, endTime)
	if err != nil {
		return nil, 0, err
	}

	return jobs, total, nil
}

// GetPendingJobsForRetry 获取pending状态且创建时间超过指定时长的任务
// 用于定时任务重试分发
func (r *PrintJobRepository) GetPendingJobsForRetry(minAge time.Duration) ([]*models.PrintJob, error) {
	query := `
		SELECT pj.id, pj.name, pj.status, pj.printer_id, p.name as printer_name, p.edge_node_id,
			   pj.user_id, pj.user_name, pj.file_path, pj.file_url, pj.file_size, pj.page_count,
			   pj.copies, pj.paper_size, pj.color_mode, pj.duplex_mode,
			   pj.start_time, pj.end_time, pj.error_message, pj.retry_count,
			   pj.max_retries, pj.created_at, pj.updated_at
		FROM print_jobs pj
		JOIN printers p ON pj.printer_id = p.id
		WHERE pj.status = 'pending'
		AND pj.created_at < $1
		ORDER BY pj.created_at ASC
		LIMIT 100`

	cutoffTime := time.Now().Add(-minAge)
	rows, err := r.db.DB.Query(query, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.PrintJob
	for rows.Next() {
		job := &models.PrintJob{}
		var userID sql.NullString
		var edgeNodeID string

		err := rows.Scan(
			&job.ID, &job.Name, &job.Status, &job.PrinterID, &job.PrinterName, &edgeNodeID,
			&userID, &job.UserName, &job.FilePath, &job.FileURL, &job.FileSize, &job.PageCount,
			&job.Copies, &job.PaperSize, &job.ColorMode, &job.DuplexMode,
			&job.StartTime, &job.EndTime, &job.ErrorMessage, &job.RetryCount,
			&job.MaxRetries, &job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if userID.Valid {
			job.UserID = userID.String
		}
		job.EdgeNodeID = edgeNodeID

		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}
