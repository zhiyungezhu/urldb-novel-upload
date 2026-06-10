package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pan "github.com/zhiyungezhu/urldb-novel-upload/common"
	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// NovelUploadProcessor 小说文件夹上传处理器
// 将一个小说文件夹（含 .txt/.epub/.pdf/bulk_txt）整体上传到夸克网盘�?
// �?在夸克建同名文件�?�?�?批量上传所有文�?�?�?分享文件�?�?�?12小时后删本地文件
type NovelUploadProcessor struct {
	repoMgr *repo.RepositoryManager
}

// NewNovelUploadProcessor 创建小说上传处理�?
func NewNovelUploadProcessor(repoMgr *repo.RepositoryManager) *NovelUploadProcessor {
	return &NovelUploadProcessor{
		repoMgr: repoMgr,
	}
}

// GetTaskType 获取任务类型
func (np *NovelUploadProcessor) GetTaskType() string {
	return "novel_upload"
}

// NovelUploadInput 小说上传任务输入
type NovelUploadInput struct {
	FolderPath string `json:"folder_path"` // 本地小说文件夹路径，�?./output/斗破苍穹/
	NovelName  string `json:"novel_name"`  // 小说名称（用于夸克文件夹�?分享标题�?
	CkID       uint   `json:"ck_id"`       // 夸克账号ID
	ParentFid  string `json:"parent_fid"`  // 夸克父目录ID（用户手动创建的小说分类目录�?
}

// NovelUploadOutput 小说上传任务输出
type NovelUploadOutput struct {
	Success      bool   `json:"success"`
	NovelName    string `json:"novel_name"`
	FolderFid    string `json:"folder_fid,omitempty"`
	FileCount    int    `json:"file_count"`
	UploadedCount int   `json:"uploaded_count"`
	ShareURL     string `json:"share_url,omitempty"`
	ShareCode    string `json:"share_code,omitempty"`
	ResourceID   uint   `json:"resource_id,omitempty"`
	Error        string `json:"error,omitempty"`
	Time         string `json:"time"`
}

// Process 处理小说上传任务�?
func (np *NovelUploadProcessor) Process(ctx context.Context, taskID uint, item *entity.TaskItem) error {
	startTime := time.Now()
	utils.InfoWithFields(map[string]interface{}{
		"task_item_id": item.ID,
		"task_id":      taskID,
	}, "开始处理小说上传任务项: %d", item.ID)

	// 解析输入
	var input NovelUploadInput
	if err := json.Unmarshal([]byte(item.InputData), &input); err != nil {
		utils.Error("解析小说上传输入数据失败: %v", err)
		return fmt.Errorf("解析小说上传输入数据失败: %v", err)
	}

	if input.FolderPath == "" {
		return fmt.Errorf("文件夹路径不能为�?)
	}
	if input.NovelName == "" {
		input.NovelName = filepath.Base(input.FolderPath)
	}
	if input.CkID == 0 {
		return fmt.Errorf("账号ID不能为空")
	}
	if input.ParentFid == "" {
		input.ParentFid = "0"
	}

	utils.Info("小说: %s, 本地路径: %s, 父目�? %s", input.NovelName, input.FolderPath, input.ParentFid)

	// 检查本地文件夹是否存在
	dirInfo, err := os.Stat(input.FolderPath)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("本地文件夹不存在: %v", err))
	}
	if !dirInfo.IsDir() {
		return np.fail(item, input, "路径不是文件�?)
	}

	// 收集文件夹内所有文件（递归，支�?bulk_txt 子目录）
	var files []string
	filepath.Walk(input.FolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})

	if len(files) == 0 {
		return np.fail(item, input, "文件夹内没有文件")
	}
	utils.Info("发现 %d 个文件待上传", len(files))

	// 获取账号信息
	cks, err := np.repoMgr.CksRepository.FindByID(input.CkID)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("获取账号信息失败: %v", err))
	}

	// 创建网盘服务
	factory := pan.NewPanFactory()
	service, err := factory.CreatePanServiceByType(pan.Quark, &pan.PanConfig{
		Cookie: cks.Ck,
	})
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("创建网盘服务失败: %v", err))
	}
	service.SetCKSRepository(np.repoMgr.CksRepository, *cks)

	// 校验 Cookie
	utils.Info("验证账号Cookie有效�?..")
	userInfo, err := service.GetUserInfo(&cks.Ck)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("Cookie验证失败（可能已过期�? %v", err))
	}
	utils.Info("Cookie有效，用户名: %s", userInfo.Username)

	// �?在夸克网盘创建小说文件夹
	utils.Info("在夸克网盘创建文件夹: %s", input.NovelName)
	novelFolderFid, err := service.Mkdir(input.ParentFid, input.NovelName)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("创建夸克文件夹失�? %v", err))
	}
	utils.Info("文件夹创建成功，fid: %s", novelFolderFid)

	// �?逐个上传文件到该文件�?
	uploadedCount := 0
	for _, filePath := range files {
		select {
		case <-ctx.Done():
			utils.Warn("任务被取消，已上�?%d/%d 个文�?, uploadedCount, len(files))
			return ctx.Err()
		default:
		}

		fileName := filepath.Base(filePath)
		utils.Info("上传文件 %d/%d: %s", uploadedCount+1, len(files), fileName)

		uploadResult, err := service.UploadFile(filePath, novelFolderFid)
		if err != nil {
			utils.Error("文件上传失败: %s, 错误: %v", fileName, err)
			continue
		}
		if uploadResult == nil || !uploadResult.Success {
			errMsg := "上传返回失败"
			if uploadResult != nil {
				errMsg = uploadResult.Message
			}
			utils.Error("文件上传返回失败: %s, msg: %s", fileName, errMsg)
			continue
		}
		uploadedCount++
	}

	if uploadedCount == 0 {
		return np.fail(item, input, "所有文件上传失�?)
	}
	utils.Info("文件上传完成: %d/%d 成功", uploadedCount, len(files))

	// �?分享文件�?
	utils.Info("开始分享文件夹: %s", input.NovelName)
	passwordResult, err := service.ShareFolder(novelFolderFid, input.NovelName)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("分享文件夹失�? %v", err))
	}

	shareURL := passwordResult.ShareURL
	shareCode := passwordResult.Code
	utils.Info("文件夹分享成�? %s �?%s (提取�? %s)", input.NovelName, shareURL, shareCode)

	// 获取 panID
	panIDInt, _ := np.repoMgr.PanRepository.FindIdByServiceType("quark")
	panID := uint(panIDInt)

	// �?保存�?resources �?
	resource := &entity.Resource{
		Title:     fmt.Sprintf("[小说] %s (%d文件)", input.NovelName, uploadedCount),
		URL:       shareURL,
		SaveURL:   shareURL,
		PanID:     &panID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsValid:   true,
		IsPublic:  true,
		CkID:      &input.CkID,
	}

	if err := np.repoMgr.ResourceRepository.Create(resource); err != nil {
		utils.Error("保存资源记录失败: %v", err)
	} else {
		utils.Info("资源已保存，ID: %d", resource.ID)
	}

	// �?12 小时后自动删除本地文件夹
	np.scheduleDelayedCleanup(input.FolderPath, 12*time.Hour)

	// 构建成功输出
	output := NovelUploadOutput{
		Success:       true,
		NovelName:     input.NovelName,
		FolderFid:     novelFolderFid,
		FileCount:     len(files),
		UploadedCount: uploadedCount,
		ShareURL:      shareURL,
		ShareCode:     shareCode,
		ResourceID:    resource.ID,
		Time:          time.Now().Format("2006-01-02 15:04:05"),
	}

	outputJSON, _ := json.Marshal(output)
	item.OutputData = string(outputJSON)

	elapsed := time.Since(startTime)
	utils.InfoWithFields(map[string]interface{}{
		"task_item_id":   item.ID,
		"novel_name":     input.NovelName,
		"file_count":     len(files),
		"uploaded_count": uploadedCount,
		"share_url":      shareURL,
		"duration_ms":    elapsed.Milliseconds(),
	}, "小说上传完成: %s, 成功 %d/%d, 耗时: %v", input.NovelName, uploadedCount, len(files), elapsed)

	return nil
}

// fail 快速构建失败输出并返回 error
func (np *NovelUploadProcessor) fail(item *entity.TaskItem, input NovelUploadInput, errMsg string) error {
	output := NovelUploadOutput{
		Success:   false,
		NovelName: input.NovelName,
		Error:     errMsg,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
	}
	outputJSON, _ := json.Marshal(output)
	item.OutputData = string(outputJSON)
	utils.Error("小说上传失败 [%s]: %s", input.NovelName, errMsg)
	return fmt.Errorf(errMsg)
}

// scheduleDelayedCleanup 延迟删除本地文件�?
func (np *NovelUploadProcessor) scheduleDelayedCleanup(folderPath string, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		absPath, _ := filepath.Abs(folderPath)
		if err := os.RemoveAll(absPath); err != nil {
			utils.Warn("延迟删除本地文件夹失�? %s, 错误: %v", absPath, err)
		} else {
			utils.Info("已清理本地文件夹: %s (延迟 %.0f小时)", absPath, delay.Hours())
		}
	}()
}

// scheduleDelayedCleanup 延迟删除本地文件�