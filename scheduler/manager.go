package scheduler

import (
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// Manager ������������
type Manager struct {
	baseScheduler          *BaseScheduler
	hotDramaScheduler      *HotDramaScheduler
	readyResourceScheduler *ReadyResourceScheduler
	sitemapScheduler       *SitemapScheduler
	googleIndexScheduler   *GoogleIndexScheduler
	uploadWatcher          *UploadWatcher
	novelUploadWatcher     *NovelUploadWatcher
}

// NewManager ����������������
func NewManager(
	hotDramaRepo repo.HotDramaRepository,
	readyResourceRepo repo.ReadyResourceRepository,
	resourceRepo repo.ResourceRepository,
	systemConfigRepo repo.SystemConfigRepository,
	panRepo repo.PanRepository,
	cksRepo repo.CksRepository,
	tagRepo repo.TagRepository,
	categoryRepo repo.CategoryRepository,
	taskItemRepo repo.TaskItemRepository,
	taskRepo repo.TaskRepository,
) *Manager {
	// ��������������
	baseScheduler := NewBaseScheduler(
		hotDramaRepo,
		readyResourceRepo,
		resourceRepo,
		systemConfigRepo,
		panRepo,
		cksRepo,
		tagRepo,
		categoryRepo,
	)

	// ������������ĵ�����
	hotDramaScheduler := NewHotDramaScheduler(baseScheduler)
	readyResourceScheduler := NewReadyResourceScheduler(baseScheduler)
	sitemapScheduler := NewSitemapScheduler(baseScheduler)
	googleIndexScheduler := NewGoogleIndexScheduler(baseScheduler, taskItemRepo, taskRepo)
	uploadWatcher := NewUploadWatcher(baseScheduler, taskRepo, taskItemRepo)
	novelUploadWatcher := NewNovelUploadWatcher(baseScheduler, taskRepo, taskItemRepo)

	return &Manager{
		baseScheduler:          baseScheduler,
		hotDramaScheduler:      hotDramaScheduler,
		readyResourceScheduler: readyResourceScheduler,
		sitemapScheduler:       sitemapScheduler,
		googleIndexScheduler:   googleIndexScheduler,
		uploadWatcher:          uploadWatcher,
		novelUploadWatcher:     novelUploadWatcher,
	}
}

// StartAll �������е�������
func (m *Manager) StartAll() {
	utils.Debug("�������е�������")

	// �����Ȳ��綨ʱ����
	m.StartHotDramaScheduler()

	// ������������Դ��������
	m.readyResourceScheduler.Start()

	// ����Google������������
	m.googleIndexScheduler.Start()

	utils.Debug("���е�������������")
}

// StopAll ֹͣ���е�������
func (m *Manager) StopAll() {
	utils.Debug("ֹͣ���е�������")

	// ֹͣ�Ȳ��綨ʱ����
	m.StopHotDramaScheduler()

	// ֹͣ��������Դ��������
	m.readyResourceScheduler.Stop()

	// ֹͣGoogle������������
	m.googleIndexScheduler.Stop()

	utils.Debug("���е���������ֹͣ")
}

// StartHotDramaScheduler �����Ȳ����������
func (m *Manager) StartHotDramaScheduler() {
	m.hotDramaScheduler.Start()
}

// StopHotDramaScheduler ֹͣ�Ȳ����������
func (m *Manager) StopHotDramaScheduler() {
	m.hotDramaScheduler.Stop()
}

// IsHotDramaRunning ����Ȳ�����������Ƿ���������
func (m *Manager) IsHotDramaRunning() bool {
	return m.hotDramaScheduler.IsRunning()
}

// StartReadyResourceScheduler ������������Դ��������
func (m *Manager) StartReadyResourceScheduler() {
	m.readyResourceScheduler.Start()
}

// StopReadyResourceScheduler ֹͣ��������Դ��������
func (m *Manager) StopReadyResourceScheduler() {
	m.readyResourceScheduler.Stop()
}

// IsReadyResourceRunning ����������Դ���������Ƿ���������
func (m *Manager) IsReadyResourceRunning() bool {
	return m.readyResourceScheduler.IsReadyResourceRunning()
}

// GetHotDramaNames ��ȡ�Ȳ��������б�
func (m *Manager) GetHotDramaNames() ([]string, error) {
	return m.hotDramaScheduler.GetHotDramaNames()
}

// StartSitemapScheduler ����Sitemap��������
func (m *Manager) StartSitemapScheduler() {
	m.sitemapScheduler.Start()
}

// StopSitemapScheduler ֹͣSitemap��������
func (m *Manager) StopSitemapScheduler() {
	m.sitemapScheduler.Stop()
}

// IsSitemapRunning ���Sitemap���������Ƿ�������
func (m *Manager) IsSitemapRunning() bool {
	return m.sitemapScheduler.IsRunning()
}

// GetSitemapConfig ��ȡSitemap����
func (m *Manager) GetSitemapConfig() (bool, error) {
	return m.sitemapScheduler.GetSitemapConfig()
}

// UpdateSitemapConfig ����Sitemap����
func (m *Manager) UpdateSitemapConfig(enabled bool) error {
	return m.sitemapScheduler.UpdateSitemapConfig(enabled)
}

// TriggerSitemapGeneration �ֶ�����sitemap����
func (m *Manager) TriggerSitemapGeneration() {
	go m.sitemapScheduler.generateSitemap()
}

// StartGoogleIndexScheduler ����Google������������
func (m *Manager) StartGoogleIndexScheduler() {
	m.googleIndexScheduler.Start()
}

// StopGoogleIndexScheduler ֹͣGoogle������������
func (m *Manager) StopGoogleIndexScheduler() {
	m.googleIndexScheduler.Stop()
}

// IsGoogleIndexRunning ���Google�������������Ƿ�������
func (m *Manager) IsGoogleIndexRunning() bool {
	return m.googleIndexScheduler.IsRunning()
}

// GetStatus ��ȡ���е��������״̬
func (m *Manager) GetStatus() map[string]bool {
	return map[string]bool{
		"hot_drama":      m.IsHotDramaRunning(),
		"ready_resource": m.IsReadyResourceRunning(),
		"sitemap":        m.IsSitemapRunning(),
		"google_index":   m.IsGoogleIndexRunning(),
		"upload_watcher":       m.IsUploadWatcherRunning(),
		"novel_upload_watcher": m.IsNovelUploadWatcherRunning(),
	}
}

// StartUploadWatcher �����ϴ�Ŀ¼���
func (m *Manager) StartUploadWatcher() {
	m.uploadWatcher.Start()
}

// StopUploadWatcher ֹͣ�ϴ�Ŀ¼���
func (m *Manager) StopUploadWatcher() {
	m.uploadWatcher.Stop()
}

// IsUploadWatcherRunning ����ϴ�Ŀ¼����Ƿ���������
func (m *Manager) IsUploadWatcherRunning() bool {
	return m.uploadWatcher.IsRunning()
}

// StartNovelUploadWatcher ����С˵�ϴ�Ŀ¼���
func (m *Manager) StartNovelUploadWatcher() {
	m.novelUploadWatcher.Start()
}

// StopNovelUploadWatcher ֹͣС˵�ϴ�Ŀ¼���
func (m *Manager) StopNovelUploadWatcher() {
	m.novelUploadWatcher.Stop()
}

// IsNovelUploadWatcherRunning ���С˵�ϴ�Ŀ¼����Ƿ���������
func (m *Manager) IsNovelUploadWatcherRunning() bool {
	return m.novelUploadWatcher.IsRunning()
}
