package handlers

import (
	"net/http"

	"github.com/zhiyungezhu/urldb-novel-upload/scheduler"
	"github.com/gin-gonic/gin"
)

// GetSchedulerStatus ��ȡ������״̬
func GetSchedulerStatus(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)

	status := gin.H{
		"hot_drama_scheduler_running":      scheduler.IsHotDramaSchedulerRunning(),
		"ready_resource_scheduler_running": scheduler.IsReadyResourceRunning(),
		"google_index_scheduler_running":   scheduler.IsGoogleIndexSchedulerRunning(),
		"sitemap_scheduler_running":        scheduler.IsSitemapSchedulerRunning(),
		"upload_watcher_running":           scheduler.IsUploadWatcherRunning(),
		"novel_upload_watcher_running":     scheduler.IsNovelUploadWatcherRunning(),
	}

	SuccessResponse(c, status)
}

// �����Ȳ��綨ʱ����
func StartHotDramaScheduler(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	if scheduler.IsHotDramaSchedulerRunning() {
		ErrorResponse(c, "�Ȳ��綨ʱ��������������", http.StatusBadRequest)
		return
	}
	scheduler.StartHotDramaScheduler()
	SuccessResponse(c, gin.H{"message": "�Ȳ��綨ʱ����������"})
}

// ֹͣ�Ȳ��綨ʱ����
func StopHotDramaScheduler(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	if !scheduler.IsHotDramaSchedulerRunning() {
		ErrorResponse(c, "�Ȳ��綨ʱ����δ������", http.StatusBadRequest)
		return
	}
	scheduler.StopHotDramaScheduler()
	SuccessResponse(c, gin.H{"message": "�Ȳ��綨ʱ������ֹͣ"})
}

// �ֶ������Ȳ��綨ʱ����
func TriggerHotDramaScheduler(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	scheduler.StartHotDramaScheduler() // ֱ������һ��
	SuccessResponse(c, gin.H{"message": "�ֶ������Ȳ��綨ʱ����ɹ�"})
}

// �ֶ���ȡ�Ȳ�������
func FetchHotDramaNames(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	names, err := scheduler.GetHotDramaNames()
	if err != nil {
		ErrorResponse(c, "��ȡ�Ȳ�������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}
	SuccessResponse(c, gin.H{"names": names})
}

// ������������Դ�Զ���������
func StartReadyResourceScheduler(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	if scheduler.IsReadyResourceRunning() {
		ErrorResponse(c, "��������Դ�Զ�������������������", http.StatusBadRequest)
		return
	}
	scheduler.StartReadyResourceScheduler()
	SuccessResponse(c, gin.H{"message": "��������Դ�Զ���������������"})
}

// ֹͣ��������Դ�Զ���������
func StopReadyResourceScheduler(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	if !scheduler.IsReadyResourceRunning() {
		ErrorResponse(c, "��������Դ�Զ���������δ������", http.StatusBadRequest)
		return
	}
	scheduler.StopReadyResourceScheduler()
	SuccessResponse(c, gin.H{"message": "��������Դ�Զ�����������ֹͣ"})
}

// �ֶ�������������Դ�Զ���������
func TriggerReadyResourceScheduler(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	scheduler.StartReadyResourceScheduler() // ֱ������һ��
	SuccessResponse(c, gin.H{"message": "�ֶ�������������Դ�Զ���������ɹ�"})
}

// �����ϴ�Ŀ¼���
func StartUploadWatcher(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	if scheduler.IsUploadWatcherRunning() {
		ErrorResponse(c, "�ϴ�Ŀ¼�������������", http.StatusBadRequest)
		return
	}
	scheduler.StartUploadWatcher()
	SuccessResponse(c, gin.H{"message": "�ϴ�Ŀ¼���������"})
}

// ֹͣ�ϴ�Ŀ¼���
func StopUploadWatcher(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	if !scheduler.IsUploadWatcherRunning() {
		ErrorResponse(c, "�ϴ�Ŀ¼���δ������", http.StatusBadRequest)
		return
	}
	scheduler.StopUploadWatcher()
	SuccessResponse(c, gin.H{"message": "�ϴ�Ŀ¼�����ֹͣ"})
}

// StartNovelUploadWatcher ����С˵�ϴ�Ŀ¼���
func StartNovelUploadWatcher(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	if scheduler.IsNovelUploadWatcherRunning() {
		ErrorResponse(c, "С˵�ϴ��������������", http.StatusBadRequest)
		return
	}
	scheduler.StartNovelUploadWatcher()
	SuccessResponse(c, gin.H{"message": "С˵�ϴ����������"})
}

// StopNovelUploadWatcher ֹͣС˵�ϴ�Ŀ¼���
func StopNovelUploadWatcher(c *gin.Context) {
	scheduler := scheduler.GetGlobalScheduler(
		repoManager.HotDramaRepository,
		repoManager.ReadyResourceRepository,
		repoManager.ResourceRepository,
		repoManager.SystemConfigRepository,
		repoManager.PanRepository,
		repoManager.CksRepository,
		repoManager.TagRepository,
		repoManager.CategoryRepository,
		repoManager.TaskItemRepository,
		repoManager.TaskRepository,
	)
	if !scheduler.IsNovelUploadWatcherRunning() {
		ErrorResponse(c, "С˵�ϴ����δ������", http.StatusBadRequest)
		return
	}
	scheduler.StopNovelUploadWatcher()
	SuccessResponse(c, gin.H{"message": "С˵�ϴ������ֹͣ"})
}
