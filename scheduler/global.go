????????package scheduler

import (
	"sync"

	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/services"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// GlobalScheduler ȫ�ֵ�����������
type GlobalScheduler struct {
	manager *Manager
	mutex   sync.RWMutex
}

var (
	globalScheduler *GlobalScheduler
	once            sync.Once
	// ȫ��Meilisearch������
	globalMeilisearchManager *services.MeilisearchManager
)

// SetGlobalMeilisearchManager ����ȫ��Meilisearch������
func SetGlobalMeilisearchManager(manager *services.MeilisearchManager) {
	globalMeilisearchManager = manager
}

// GetGlobalMeilisearchManager ��ȡȫ��Meilisearch������
func GetGlobalMeilisearchManager() *services.MeilisearchManager {
	return globalMeilisearchManager
}

// GetGlobalScheduler ��ȡȫ�ֵ�����ʵ��������ģʽ��
func GetGlobalScheduler(hotDramaRepo repo.HotDramaRepository, readyResourceRepo repo.ReadyResourceRepository, resourceRepo repo.ResourceRepository, systemConfigRepo repo.SystemConfigRepository, panRepo repo.PanRepository, cksRepo repo.CksRepository, tagRepo repo.TagRepository, categoryRepo repo.CategoryRepository, taskItemRepo repo.TaskItemRepository, taskRepo repo.TaskRepository) *GlobalScheduler {
	once.Do(func() {
		globalScheduler = &GlobalScheduler{
			manager: NewManager(hotDramaRepo, readyResourceRepo, resourceRepo, systemConfigRepo, panRepo, cksRepo, tagRepo, categoryRepo, taskItemRepo, taskRepo),
		}
	})
	return globalScheduler
}

// StartHotDramaScheduler �����Ȳ��綨ʱ����
func (gs *GlobalScheduler) StartHotDramaScheduler() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if gs.manager.IsHotDramaRunning() {
		utils.Debug("�Ȳ��綨ʱ��������������")
		return
	}

	gs.manager.StartHotDramaScheduler()
	utils.Debug("ȫ�ֵ������������Ȳ��綨ʱ����")
}

// StopHotDramaScheduler ֹͣ�Ȳ��綨ʱ����
func (gs *GlobalScheduler) StopHotDramaScheduler() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if !gs.manager.IsHotDramaRunning() {
		utils.Debug("�Ȳ��綨ʱ����δ������")
		return
	}

	gs.manager.StopHotDramaScheduler()
	utils.Debug("ȫ�ֵ�������ֹͣ�Ȳ��綨ʱ����")
}

// IsHotDramaSchedulerRunning ����Ȳ��綨ʱ�����Ƿ�������
func (gs *GlobalScheduler) IsHotDramaSchedulerRunning() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()
	return gs.manager.IsHotDramaRunning()
}

// GetHotDramaNames �ֶ���ȡ�Ȳ�������
func (gs *GlobalScheduler) GetHotDramaNames() ([]string, error) {
	return gs.manager.GetHotDramaNames()
}

// StartReadyResourceScheduler ������������Դ�Զ���������
func (gs *GlobalScheduler) StartReadyResourceScheduler() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if gs.manager.IsReadyResourceRunning() {
		utils.Debug("��������Դ�Զ�������������������")
		return
	}

	gs.manager.StartReadyResourceScheduler()
	utils.Debug("ȫ�ֵ�������������������Դ�Զ���������")
}

// StopReadyResourceScheduler ֹͣ��������Դ�Զ���������
func (gs *GlobalScheduler) StopReadyResourceScheduler() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if !gs.manager.IsReadyResourceRunning() {
		utils.Debug("��������Դ�Զ���������δ������")
		return
	}

	gs.manager.StopReadyResourceScheduler()
	utils.Debug("ȫ�ֵ�������ֹͣ��������Դ�Զ���������")
}

// IsReadyResourceRunning ����������Դ�Զ����������Ƿ�������
func (gs *GlobalScheduler) IsReadyResourceRunning() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()
	return gs.manager.IsReadyResourceRunning()
}

// UpdateSchedulerStatusWithAutoTransfer ����ϵͳ���ø��µ�����״̬�������Զ�ת�棩
func (gs *GlobalScheduler) UpdateSchedulerStatusWithAutoTransfer(autoFetchHotDramaEnabled bool, autoProcessReadyResources bool, autoTransferEnabled bool) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	// �����Ȳ����Զ���ȡ����
	if autoFetchHotDramaEnabled {
		if !gs.manager.IsHotDramaRunning() {
			utils.Info("ϵͳ���������Զ���ȡ�Ȳ��磬������ʱ����")
			gs.manager.StartHotDramaScheduler()
		}
	} else {
		if gs.manager.IsHotDramaRunning() {
			utils.Info("ϵͳ���ý����Զ���ȡ�Ȳ��磬ֹͣ��ʱ����")
			gs.manager.StopHotDramaScheduler()
		}
	}

	// ������������Դ�Զ���������
	if autoProcessReadyResources {
		if !gs.manager.IsReadyResourceRunning() {
			utils.Info("ϵͳ���������Զ�������������Դ��������ʱ����")
			gs.manager.StartReadyResourceScheduler()
		}
	} else {
		if gs.manager.IsReadyResourceRunning() {
			utils.Info("ϵͳ���ý����Զ�������������Դ��ֹͣ��ʱ����")
			gs.manager.StopReadyResourceScheduler()
		}
	}

}

// StartSitemapScheduler ����Sitemap��������
func (gs *GlobalScheduler) StartSitemapScheduler() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if gs.manager.IsSitemapRunning() {
		utils.Debug("Sitemap��ʱ��������������")
		return
	}

	gs.manager.StartSitemapScheduler()
	utils.Debug("ȫ�ֵ�����������Sitemap��ʱ����")
}

// StopSitemapScheduler ֹͣSitemap��������
func (gs *GlobalScheduler) StopSitemapScheduler() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if !gs.manager.IsSitemapRunning() {
		utils.Debug("Sitemap��ʱ����δ������")
		return
	}

	gs.manager.StopSitemapScheduler()
	utils.Debug("ȫ�ֵ�������ֹͣSitemap��ʱ����")
}

// IsSitemapSchedulerRunning ���Sitemap��ʱ�����Ƿ�������
func (gs *GlobalScheduler) IsSitemapSchedulerRunning() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()
	return gs.manager.IsSitemapRunning()
}

// UpdateSitemapConfig ����Sitemap����
func (gs *GlobalScheduler) UpdateSitemapConfig(enabled bool) error {
	return gs.manager.UpdateSitemapConfig(enabled)
}

// GetSitemapConfig ��ȡSitemap����
func (gs *GlobalScheduler) GetSitemapConfig() (bool, error) {
	return gs.manager.GetSitemapConfig()
}

// TriggerSitemapGeneration �ֶ�����sitemap����
func (gs *GlobalScheduler) TriggerSitemapGeneration() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	gs.manager.TriggerSitemapGeneration()
}

// StartGoogleIndexScheduler ����Google������������
func (gs *GlobalScheduler) StartGoogleIndexScheduler() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if gs.manager.IsGoogleIndexRunning() {
		utils.Debug("Google����������������������")
		return
	}

	gs.manager.StartGoogleIndexScheduler()
	utils.Info("Google������������������")
}

// StopGoogleIndexScheduler ֹͣGoogle������������
func (gs *GlobalScheduler) StopGoogleIndexScheduler() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if !gs.manager.IsGoogleIndexRunning() {
		utils.Debug("Google������������δ������")
		return
	}

	gs.manager.StopGoogleIndexScheduler()
	utils.Info("Google��������������ֹͣ")
}

// IsGoogleIndexSchedulerRunning ���Google�������������Ƿ�������
func (gs *GlobalScheduler) IsGoogleIndexSchedulerRunning() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()

	return gs.manager.IsGoogleIndexRunning()
}

// StartUploadWatcher �����ϴ�Ŀ¼���
func (gs *GlobalScheduler) StartUploadWatcher() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if gs.manager.IsUploadWatcherRunning() {
		utils.Debug("�ϴ�Ŀ¼�������������")
		return
	}

	gs.manager.StartUploadWatcher()
	utils.Debug("ȫ�ֵ������������ϴ�Ŀ¼���")
}

// StopUploadWatcher ֹͣ�ϴ�Ŀ¼���
func (gs *GlobalScheduler) StopUploadWatcher() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if !gs.manager.IsUploadWatcherRunning() {
		utils.Debug("�ϴ�Ŀ¼���δ������")
		return
	}

	gs.manager.StopUploadWatcher()
	utils.Debug("ȫ�ֵ�������ֹͣ�ϴ�Ŀ¼���")
}

// IsUploadWatcherRunning ����ϴ�Ŀ¼����Ƿ���������
func (gs *GlobalScheduler) IsUploadWatcherRunning() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()
	return gs.manager.IsUploadWatcherRunning()
}

// StartNovelUploadWatcher ����С˵�ϴ�Ŀ¼���
func (gs *GlobalScheduler) StartNovelUploadWatcher() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if gs.manager.IsNovelUploadWatcherRunning() {
		utils.Debug("С˵�ϴ��������������")
		return
	}

	gs.manager.StartNovelUploadWatcher()
	utils.Debug("ȫ�ֵ�����������С˵�ϴ����")
}

// StopNovelUploadWatcher ֹͣС˵�ϴ�Ŀ¼���
func (gs *GlobalScheduler) StopNovelUploadWatcher() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()

	if !gs.manager.IsNovelUploadWatcherRunning() {
		utils.Debug("С˵�ϴ����δ������")
		return
	}

	gs.manager.StopNovelUploadWatcher()
	utils.Debug("ȫ�ֵ�������ֹͣС˵�ϴ����")
}

// IsNovelUploadWatcherRunning ���С˵�ϴ�����Ƿ���������
func (gs *GlobalScheduler) IsNovelUploadWatcherRunning() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()
	return gs.manager.IsNovelUploadWatcherRunning()
}
