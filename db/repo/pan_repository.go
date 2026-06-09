package repo

import (
	"fmt"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"

	"gorm.io/gorm"
)

// PanRepository Pan的Repository接口
type PanRepository interface {
	BaseRepository[entity.Pan]
	FindWithCks() ([]entity.Pan, error)
	FindIdByServiceType(serviceType string) (int, error)
}

// PanRepositoryImpl Pan的Repository实现
type PanRepositoryImpl struct {
	BaseRepositoryImpl[entity.Pan]
}

// NewPanRepository 创建Pan Repository
func NewPanRepository(db *gorm.DB) PanRepository {
	return &PanRepositoryImpl{
		BaseRepositoryImpl: BaseRepositoryImpl[entity.Pan]{db: db},
	}
}

// FindWithCks 查找包含Cks的Pan
func (r *PanRepositoryImpl) FindWithCks() ([]entity.Pan, error) {
	var pans []entity.Pan
	err := r.db.Preload("Cks").Find(&pans).Error
	return pans, err
}

func (r *PanRepositoryImpl) FindIdByServiceType(serviceType string) (int, error) {
	var pan entity.Pan
	err := r.db.Where("name = ?", serviceType).Find(&pan).Error
	if err != nil {
		return 0, fmt.Errorf("获取panId失败： %v", serviceType)
	}
	return int(pan.ID), nil
}
