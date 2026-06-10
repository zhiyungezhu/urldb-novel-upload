package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// NovelUploadWatcher 小说上传目录监控�?
// 定时扫描小说下载输出目录，发现完成的小说文件夹自动创建上传任务：
//   - 扫描一级子目录
//   - 判定标准：目录中包含 .txt/.epub/.pdf 文件，且 5 分钟内无文件修改（认为下载完成）
//   - 每处理完一个目录就创建 novel_upload 任务
type NovelUploadWatcher struct {
	base         *BaseScheduler
	taskRepo     repo.TaskRepository
	taskItemRepo repo.TaskItemRepository
	watchDir     string
	intervalMin  int    // 扫描间隔（分钟）
	ckID         uint   // 夸克账号ID
	parentFid    string // 夸克父目录（小说分类根目录）
	completedDir map[string]bool // 已处理的目录（防重复�?
}

// NewNovelUploadWatcher 创建小说上传监控�?
func NewNovelUploadWatcher(
	base *BaseScheduler,
	taskRepo repo.TaskRepository,
	taskItemRepo repo.TaskItemRepository,
) *NovelUploadWatcher {
	watchDir := getEnvOrDefault("NOVEL_WATCH_DIR", "./watch_novel_output")
	ckIDStr := os.Getenv("NOVEL_CK_ID")
	parentFid := os.Getenv("NOVEL_PARENT_FID")

	var ckID uint
	if ckIDStr != "" {
		fmt.Sscanf(ckIDStr, "%d", &ckID)
	}

	if parentFid == "" {
		parentFid = "0"
	}

	if _, err := os.Stat(watchDir); os.IsNotExist(err) {
		os.MkdirAll(watchDir, 0755)
	}

	return &NovelUploadWatcher{
		base:         base,
		taskRepo:     taskRepo,
		taskItemRepo: taskItemRepo,
		watchDir:     watchDir,
		intervalMin:  5,
		ckID:         ckID,
		parentFid:    parentFid,
		completedDir: make(map[string]bool),
	}
}

// Start 启动监控
func (nw *NovelUploadWatcher) Start() {
	if nw.base.IsRunning() {
		utils.Debug("小说上传监控已在运行�?)
		return
	}

	nw.base.SetRunning(true)
	utils.Info("小说上传监控已启动，监控目录: %s, 父目�? %s", nw.watchDir, nw.parentFid)

	go nw.runWatcher()
}

// Stop 停止监控
func (nw *NovelUploadWatcher) Stop() {
	if !nw.base.IsRunning() {
		utils.Debug("小说上传监控未在运行")
		return
	}

	nw.base.SetRunning(false)
	nw.base.GetStopChan() <- true
	utils.Info("小说上传监控已停�?)
}

// IsRunning 检查是否正在运�?
func (nw *NovelUploadWatcher) IsRunning() bool {
	return nw.base.IsRunning()
}

// runWatcher 运行监控循环
func (nw *NovelUploadWatcher) runWatcher() {
	defer nw.base.SetRunning(false)

	for {
		nw.scanAndProcess()

		if nw.base.SleepWithStopCheck(time.Duration(nw.intervalMin) * time.Minute) {
			return
		}
	}
}

// scanAndProcess 扫描目录并处理完成的小说文件�?
func (nw *NovelUploadWatcher) scanAndProcess() {
	entries, err := os.ReadDir(nw.watchDir)
	if err != nil {
		utils.Error("扫描小说输出目录失败: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		novelDirPath := filepath.Join(nw.watchDir, entry.Name())
		novelName := entry.Name()

		// 跳过已处理的目录
		if nw.completedDir[novelDirPath] {
			continue
		}

		// 检查目录是否下载完�?
		if !nw.isDownloadComplete(novelDirPath) {
			continue
		}

		utils.Info("发现已完成的小说下载目录: %s", novelName)

		// 创建上传任务
		if err := nw.createNovelUploadTask(novelDirPath, novelName); err != nil {
			utils.Error("创建小说上传任务失败 [%s]: %v", novelName, err)
			continue
		}

		nw.completedDir[novelDirPath] = true
	}
}

// isDownloadComplete 判断小说文件夹是否下载完�?
// 条件：包�?.txt/.epub/.pdf 文件 + 最�?5 分钟内无文件修改
func (nw *NovelUploadWatcher) isDownloadComplete(dirPath string) bool {
	hasNovelFile := false
	now := time.Now()
	fiveMinAgo := now.Add(-5 * time.Minute)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// 检查是否包含小说文件格�?
		switch filepath.Ext(info.Name()) {
		case ".txt", ".epub", ".pdf":
			hasNovelFile = true
		}

		// 有文件在5分钟内修改过 �?下载还在进行�?
		if info.ModTime().After(fiveMinAgo) {
			return fmt.Errorf("文件仍在更新")
		}

		return nil
	})

	if err != nil {
		return false // 有文件在更新�?
	}

	return hasNovelFile
}

// createNovelUploadTask 为单个小说文件夹创建 novel_upload 任务
func (nw *NovelUploadWatcher) createNovelUploadTask(novelDirPath, novelName string) error {
	if nw.ckID == 0 {
		return fmt.Errorf("未配置小说上传账号ID（请设置环境变量 NOVEL_CK_ID�?)
	}

	now := time.Now()

	// 构建输入
	input := map[string]interface{}{
		"folder_path": novelDirPath,
		"novel_name":  novelName,
		"ck_id":       nw.ckID,
		"parent_fid":  nw.parentFid,
	}
	inputJSON, _ := json.Marshal(input)

	// 创建 novel_upload 任务项（独立任务，每个小说一个）
	taskTitle := fmt.Sprintf("小说上传 - %s", novelName)
	config := map[string]interface{}{
		"selected_accounts": []uint{nw.ckID},
		"auto_created":      true,
		"watch_dir":         nw.watchDir,
		"parent_fid":        nw.parentFid,
	}
	configJSON, _ := json.Marshal(config)

	newTask := &entity.Task{
		Title:       taskTitle,
		Description: fmt.Sprintf("自动扫描 %s 目录，发现小�?%s", nw.watchDir, novelName),
		Type:        "novel_upload",
		Status:      "pending",
		TotalItems:  1,
		Config:      string(configJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := nw.taskRepo.Create(newTask); err != nil {
		return fmt.Errorf("创建小说上传任务失败: %v", err)
	}

	// 创建任务�?
	taskItem := &entity.TaskItem{
		TaskID:    newTask.ID,
		Status:    "pending",
		InputData: string(inputJSON),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := nw.taskItemRepo.Create(taskItem); err != nil {
		return fmt.Errorf("创建小说上传任务项失�? %v", err)
	}

	utils.Info("小说上传任务创建成功: %s, 任务ID: %d", novelName, newTask.ID)
	return nil
}
