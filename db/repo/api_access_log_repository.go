package repo

import (
	"encoding/json"
	"time"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"

	"gorm.io/gorm"
)

// APIAccessLogRepository API访问日志Repository接口
type APIAccessLogRepository interface {
	BaseRepository[entity.APIAccessLog]
	RecordAccess(ip, userAgent, endpoint, method string, requestParams interface{}, responseStatus int, responseData interface{}, processCount int, errorMessage string, processingTime int64) error
	GetSummary() (*entity.APIAccessLogSummary, error)
	GetStatsByEndpoint() ([]entity.APIAccessLogStats, error)
	FindWithFilters(page, limit int, startDate, endDate *time.Time, endpoint, ip string) ([]entity.APIAccessLog, int64, error)
	ClearOldLogs(days int) error
}

// APIAccessLogRepositoryImpl API访问日志Repository实现
type APIAccessLogRepositoryImpl struct {
	BaseRepositoryImpl[entity.APIAccessLog]
}

// NewAPIAccessLogRepository 创建API访问日志Repository
func NewAPIAccessLogRepository(db *gorm.DB) APIAccessLogRepository {
	return &APIAccessLogRepositoryImpl{
		BaseRepositoryImpl: BaseRepositoryImpl[entity.APIAccessLog]{db: db},
	}
}

// RecordAccess 记录API访问
func (r *APIAccessLogRepositoryImpl) RecordAccess(ip, userAgent, endpoint, method string, requestParams interface{}, responseStatus int, responseData interface{}, processCount int, errorMessage string, processingTime int64) error {
	log := entity.APIAccessLog{
		IP:             ip,
		UserAgent:      userAgent,
		Endpoint:       endpoint,
		Method:         method,
		ResponseStatus: responseStatus,
		ProcessCount:   processCount,
		ErrorMessage:   errorMessage,
		ProcessingTime: processingTime,
	}

	// 序列化请求参数
	if requestParams != nil {
		if paramsJSON, err := json.Marshal(requestParams); err == nil {
			log.RequestParams = string(paramsJSON)
		}
	}

	// 序列化响应数据（限制大小，避免存储大量数据）
	if responseData != nil {
		if dataJSON, err := json.Marshal(responseData); err == nil {
			// 限制响应数据长度，避免存储过多数据
			dataStr := string(dataJSON)
			if len(dataStr) > 2000 {
				dataStr = dataStr[:2000] + "..."
			}
			log.ResponseData = dataStr
		}
	}

	return r.db.Create(&log).Error
}

// GetSummary 获取访问日志汇总
func (r *APIAccessLogRepositoryImpl) GetSummary() (*entity.APIAccessLogSummary, error) {
	var summary entity.APIAccessLogSummary
	now := utils.GetCurrentTime()
	todayStr := now.Format(utils.TimeFormatDate)
	weekStart := now.AddDate(0, 0, -int(now.Weekday())+1).Format(utils.TimeFormatDate)
	monthStart := now.Format("2006-01") + "-01"

	// 总请求数
	if err := r.db.Model(&entity.APIAccessLog{}).Count(&summary.TotalRequests).Error; err != nil {
		return nil, err
	}

	// 今日请求数
	if err := r.db.Model(&entity.APIAccessLog{}).Where("DATE(created_at) = ?", todayStr).Count(&summary.TodayRequests).Error; err != nil {
		return nil, err
	}

	// 本周请求数
	if err := r.db.Model(&entity.APIAccessLog{}).Where("created_at >= ?", weekStart).Count(&summary.WeekRequests).Error; err != nil {
		return nil, err
	}

	// 本月请求数
	if err := r.db.Model(&entity.APIAccessLog{}).Where("created_at >= ?", monthStart).Count(&summary.MonthRequests).Error; err != nil {
		return nil, err
	}

	// 错误请求数
	if err := r.db.Model(&entity.APIAccessLog{}).Where("response_status >= 400").Count(&summary.ErrorRequests).Error; err != nil {
		return nil, err
	}

	// 唯一IP数
	if err := r.db.Model(&entity.APIAccessLog{}).Distinct("ip").Count(&summary.UniqueIPs).Error; err != nil {
		return nil, err
	}

	return &summary, nil
}

// GetStatsByEndpoint 按端点获取统计
func (r *APIAccessLogRepositoryImpl) GetStatsByEndpoint() ([]entity.APIAccessLogStats, error) {
	var stats []entity.APIAccessLogStats

	query := `
		SELECT
			endpoint,
			method,
			COUNT(*) as request_count,
			SUM(CASE WHEN response_status >= 400 THEN 1 ELSE 0 END) as error_count,
			AVG(processing_time) as avg_process_time,
			MAX(created_at) as last_access
		FROM api_access_logs
		WHERE deleted_at IS NULL
		GROUP BY endpoint, method
		ORDER BY request_count DESC
	`

	err := r.db.Raw(query).Scan(&stats).Error
	return stats, err
}

// FindWithFilters 带过滤条件的分页查找访问日志
func (r *APIAccessLogRepositoryImpl) FindWithFilters(page, limit int, startDate, endDate *time.Time, endpoint, ip string) ([]entity.APIAccessLog, int64, error) {
	var logs []entity.APIAccessLog
	var total int64

	offset := (page - 1) * limit
	query := r.db.Model(&entity.APIAccessLog{})

	// 添加过滤条件
	if startDate != nil {
		query = query.Where("created_at >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("created_at <= ?", *endDate)
	}
	if endpoint != "" {
		query = query.Where("endpoint LIKE ?", "%"+endpoint+"%")
	}
	if ip != "" {
		query = query.Where("ip = ?", ip)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据，按创建时间倒序排列
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

// ClearOldLogs 清理旧日志
func (r *APIAccessLogRepositoryImpl) ClearOldLogs(days int) error {
	cutoffDate := utils.GetCurrentTime().AddDate(0, 0, -days)
	return r.db.Where("created_at < ?", cutoffDate).Delete(&entity.APIAccessLog{}).Error
}
