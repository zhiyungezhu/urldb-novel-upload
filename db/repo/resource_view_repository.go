package repo

import (
	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
	"gorm.io/gorm"
)

// ResourceViewRepository 资源访问记录仓库接口
type ResourceViewRepository interface {
	BaseRepository[entity.ResourceView]
	RecordView(resourceID uint, ipAddress, userAgent string) error
	GetTodayViews() (int64, error)
	GetViewsByDate(date string) (int64, error)
	GetViewsTrend(days int) ([]map[string]interface{}, error)
	GetResourceViews(resourceID uint, limit int) ([]entity.ResourceView, error)
}

// ResourceViewRepositoryImpl 资源访问记录仓库实现
type ResourceViewRepositoryImpl struct {
	BaseRepositoryImpl[entity.ResourceView]
}

// NewResourceViewRepository 创建资源访问记录仓库
func NewResourceViewRepository(db *gorm.DB) ResourceViewRepository {
	return &ResourceViewRepositoryImpl{
		BaseRepositoryImpl: BaseRepositoryImpl[entity.ResourceView]{db: db},
	}
}

// RecordView 记录资源访问
func (r *ResourceViewRepositoryImpl) RecordView(resourceID uint, ipAddress, userAgent string) error {
	view := &entity.ResourceView{
		ResourceID: resourceID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	}
	return r.db.Create(view).Error
}

// GetTodayViews 获取今日访问量
func (r *ResourceViewRepositoryImpl) GetTodayViews() (int64, error) {
	today := utils.GetTodayString()
	var count int64
	err := r.db.Model(&entity.ResourceView{}).
		Where("DATE(created_at) = ?", today).
		Count(&count).Error
	return count, err
}

// GetViewsByDate 获取指定日期的访问量
func (r *ResourceViewRepositoryImpl) GetViewsByDate(date string) (int64, error) {
	var count int64
	err := r.db.Model(&entity.ResourceView{}).
		Where("DATE(created_at) = ?", date).
		Count(&count).Error
	return count, err
}

// GetViewsTrend 获取访问量趋势数据
func (r *ResourceViewRepositoryImpl) GetViewsTrend(days int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for i := days - 1; i >= 0; i-- {
		date := utils.GetCurrentTime().AddDate(0, 0, -i)
		dateStr := date.Format(utils.TimeFormatDate)

		count, err := r.GetViewsByDate(dateStr)
		if err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"date":  dateStr,
			"views": count,
		})
	}

	return results, nil
}

// GetResourceViews 获取指定资源的访问记录
func (r *ResourceViewRepositoryImpl) GetResourceViews(resourceID uint, limit int) ([]entity.ResourceView, error) {
	var views []entity.ResourceView
	err := r.db.Where("resource_id = ?", resourceID).
		Order("created_at DESC").
		Limit(limit).
		Find(&views).Error
	return views, err
}
