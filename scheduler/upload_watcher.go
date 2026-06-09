package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/task"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// UploadWatcher 上传目录监控�?
// 定时扫描指定目录，发现新文件自动创建上传任务
type UploadWatcher struct {
	base         *BaseScheduler
	taskRepo     repo.TaskRepository
	taskItemRepo repo.TaskItemRepository
	watchDir     string
	intervalMin  int  // 扫描间隔（分钟）
	ckID         uint // 上传使用的账号ID
	pdirFid      string // 目标目录ID
}

// NewUploadWatcher 创建上传目录监控�?
func NewUploadWatcher(
	base *BaseScheduler,
	taskRepo repo.TaskRepository,
	taskItemRepo repo.TaskItemRepository,
) *UploadWatcher {
	watchDir := getEnvOrDefault("UPLOAD_WATCH_DIR", "./pending_upload")
	ckIDStr := os.Getenv("UPLOAD_CK_ID")
	pdirFid := os.Getenv("UPLOAD_PDIR_FID")

	var ckID uint
	if ckIDStr != "" {
		fmt.Sscanf(ckIDStr, "%d", &ckID)
	}

	if pdirFid == "" {
		pdirFid = "0"
	}

	return &UploadWatcher{
		base:         base,
		taskRepo:     taskRepo,
		taskItemRepo: taskItemRepo,
		watchDir:     watchDir,
		intervalMin:  5, // 默认5分钟扫描一�?
		ckID:         ckID,
		pdirFid:      pdirFid,
	}
}

// Start 启动监控
func (uw *UploadWatcher) Start() {
	if uw.base.IsRunning() {
		utils.Debug("上传目录监控已在运行�?)
		return
	}

	uw.base.SetRunning(true)
	utils.Info("上传目录监控已启动，监控目录: %s, 扫描间隔: %d分钟", uw.watchDir, uw.intervalMin)

	go uw.runWatcher()
}

// Stop 停止监控
func (uw *UploadWatcher) Stop() {
	if !uw.base.IsRunning() {
		utils.Debug("上传目录监控未在运行")
		return
	}

	uw.base.SetRunning(false)
	uw.base.GetStopChan() <- true
	utils.Info("上传目录监控已停�?)
}

// IsRunning 检查是否正在运�?
func (uw *UploadWatcher) IsRunning() bool {
	return uw.base.IsRunning()
}

// runWatcher 运行监控循环
func (uw *UploadWatcher) runWatcher() {
	defer uw.base.SetRunning(false)

	for {
		// 扫描目录
		files, err := uw.scanDirectory()
		if err != nil {
			utils.Error("扫描上传目录失败: %v", err)
		} else if len(files) > 0 {
			utils.Info("发现 %d 个文件等待上�?, len(files))
			if err := uw.createUploadTasks(files); err != nil {
				utils.Error("创建上传任务失败: %v", err)
			}
		}

		// 等待下一次扫�?
		if uw.base.SleepWithStopCheck(time.Duration(uw.intervalMin) * time.Minute) {
			return // 收到停止信号
		}
	}
}

// scanDirectory 扫描目录获取文件列表
func (uw *UploadWatcher) scanDirectory() ([]string, error) {
	// 确保目录存在
	if err := os.MkdirAll(uw.watchDir, 0755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %v", err)
	}

	entries, err := os.ReadDir(uw.watchDir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %v", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue // 跳过子目�?
		}
		fullPath := filepath.Join(uw.watchDir, entry.Name())
		files = append(files, fullPath)
	}

	return files, nil
}

// createUploadTasks 为文件列表创建上传任�?
func (uw *UploadWatcher) createUploadTasks(files []string) error {
	if uw.ckID == 0 {
		return fmt.Errorf("未配置上传账号ID（请设置环境变量 UPLOAD_CK_ID�?)
	}

	if len(files) == 0 {
		return nil
	}

	// 创建批量上传任务
	now := time.Now()
	taskTitle := fmt.Sprintf("自动上传任务 - %s", now.Format("2006-01-02 15:04:05"))

	config := map[string]interface{}{
		"selected_accounts": []uint{uw.ckID},
		"auto_created":      true,
		"watch_dir":         uw.watchDir,
	}

	configJSON, _ := json.Marshal(config)

	newTask := &entity.Task{
		Title:       taskTitle,
		Description: fmt.Sprintf("自动扫描 %s 目录，发�?%d 个文�?, uw.watchDir, len(files)),
		Type:        "upload",
		Status:      "pending",
		TotalItems:  len(files),
		Config:      string(configJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := uw.taskRepo.Create(newTask); err != nil {
		return fmt.Errorf("创建上传任务失败: %v", err)
	}

	// 为每个文件创建任务项
	successCount := 0
	for _, filePath := range files {
		uploadInput := task.UploadInput{
			FilePath: filePath,
			CkID:     uw.ckID,
			PdirFid:  uw.pdirFid,
		}

		inputJSON, _ := json.Marshal(uploadInput)

		taskItem := &entity.TaskItem{
			TaskID:    newTask.ID,
			Status:    "pending",
			InputData: string(inputJSON),
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := uw.taskItemRepo.Create(taskItem); err != nil {
			utils.Error("创建上传任务项失�? %s, 错误: %v", filePath, err)
			continue
		}
		successCount++
	}

	utils.Info("自动上传任务创建完成: 任务ID=%d, 文件�?%d/%d", newTask.ID, successCount, len(files))
	return nil
}
