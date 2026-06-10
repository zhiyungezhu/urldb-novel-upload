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

// UploadProcessor 上传任务处理�?
type UploadProcessor struct {
	repoMgr *repo.RepositoryManager
}

// NewUploadProcessor 创建上传任务处理�?
func NewUploadProcessor(repoMgr *repo.RepositoryManager) *UploadProcessor {
	return &UploadProcessor{
		repoMgr: repoMgr,
	}
}

// GetTaskType 获取任务类型
func (up *UploadProcessor) GetTaskType() string {
	return "upload"
}

// UploadInput 上传任务输入数据
type UploadInput struct {
	FilePath string `json:"file_path"` // 本地文件路径
	CkID     uint   `json:"ck_id"`     // 账号ID
	PdirFid  string `json:"pdir_fid"`  // 目标目录ID�?0"=根目�?
}

// UploadOutput 上传任务输出数据
type UploadOutput struct {
	Success     bool   `json:"success"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ShareURL    string `json:"share_url,omitempty"`
	ResourceID  uint   `json:"resource_id,omitempty"`
	Error       string `json:"error,omitempty"`
	IsDuplicate bool   `json:"is_duplicate"`
	Time        string `json:"time"`
}

// Process 处理上传任务�?
func (up *UploadProcessor) Process(ctx context.Context, taskID uint, item *entity.TaskItem) error {
	startTime := utils.GetCurrentTime()
	utils.InfoWithFields(map[string]interface{}{
		"task_item_id": item.ID,
		"task_id":      taskID,
	}, "开始处理上传任务项: %d", item.ID)

	// 解析输入数据
	var input UploadInput
	if err := json.Unmarshal([]byte(item.InputData), &input); err != nil {
		utils.Error("解析上传输入数据失败: %v", err)
		return fmt.Errorf("解析上传输入数据失败: %v", err)
	}

	// 验证输入
	if input.FilePath == "" {
		return fmt.Errorf("文件路径不能为空")
	}
	if input.CkID == 0 {
		return fmt.Errorf("账号ID不能为空")
	}

	// 检查本地文件是否存�?
	fileInfo, err := os.Stat(input.FilePath)
	if err != nil {
		output := UploadOutput{
			Success:  false,
			FileName: filepath.Base(input.FilePath),
			Error:    fmt.Sprintf("本地文件不存�? %v", err),
			Time:     utils.GetCurrentTimeString(),
		}
		outputJSON, _ := json.Marshal(output)
		item.OutputData = string(outputJSON)
		return fmt.Errorf("本地文件不存�? %s", input.FilePath)
	}

	if fileInfo.IsDir() {
		output := UploadOutput{
			Success:  false,
			FileName: filepath.Base(input.FilePath),
			Error:    "不能上传目录",
			Time:     utils.GetCurrentTimeString(),
		}
		outputJSON, _ := json.Marshal(output)
		item.OutputData = string(outputJSON)
		return fmt.Errorf("不能上传目录: %s", input.FilePath)
	}

	fileName := filepath.Base(input.FilePath)
	fileSize := fileInfo.Size()

	utils.Info("准备上传文件: %s (大小: %d bytes)", fileName, fileSize)

	// 获取账号信息
	cks, err := up.repoMgr.CksRepository.FindByID(input.CkID)
	if err != nil {
		output := UploadOutput{
			Success:  false,
			FileName: fileName,
			FileSize: fileSize,
			Error:    fmt.Sprintf("获取账号信息失败: %v", err),
			Time:     utils.GetCurrentTimeString(),
		}
		outputJSON, _ := json.Marshal(output)
		item.OutputData = string(outputJSON)
		return fmt.Errorf("获取账号信息失败: %v", err)
	}

	// 创建网盘服务
	factory := pan.NewPanFactory()
	service, err := factory.CreatePanServiceByType(pan.Quark, &pan.PanConfig{
		Cookie: cks.Ck,
	})
	if err != nil {
		output := UploadOutput{
			Success:  false,
			FileName: fileName,
			FileSize: fileSize,
			Error:    fmt.Sprintf("创建网盘服务失败: %v", err),
			Time:     utils.GetCurrentTimeString(),
		}
		outputJSON, _ := json.Marshal(output)
		item.OutputData = string(outputJSON)
		return fmt.Errorf("创建网盘服务失败: %v", err)
	}
	service.SetCKSRepository(up.repoMgr.CksRepository, *cks)

	// 校验 Cookie 是否有效
	utils.Info("验证账号Cookie有效�?..")
	userInfo, err := service.GetUserInfo(&cks.Ck)
	if err != nil {
		output := UploadOutput{
			Success:  false,
			FileName: fileName,
			FileSize: fileSize,
			Error:    fmt.Sprintf("Cookie验证失败（可能已过期�? %v", err),
			Time:     utils.GetCurrentTimeString(),
		}
		outputJSON, _ := json.Marshal(output)
		item.OutputData = string(outputJSON)
		return fmt.Errorf("Cookie验证失败: %v", err)
	}
	utils.Info("Cookie有效，用户名: %s, 可用空间: %d bytes", userInfo.Username, userInfo.TotalSpace-userInfo.UsedSpace)

	// 执行上传
	targetDir := input.PdirFid
	if targetDir == "" {
		targetDir = "0"
	}
	utils.Info("上传目标目录: pdir_fid=%s", targetDir)
	uploadResult, err := service.UploadFile(input.FilePath, targetDir)
	if err != nil {
		output := UploadOutput{
			Success:  false,
			FileName: fileName,
			FileSize: fileSize,
			Error:    fmt.Sprintf("上传失败: %v", err),
			Time:     utils.GetCurrentTimeString(),
		}
		outputJSON, _ := json.Marshal(output)
		item.OutputData = string(outputJSON)
		utils.Error("上传文件失败: %s, 错误: %v", fileName, err)
		return fmt.Errorf("上传失败: %v", err)
	}

	if uploadResult == nil || !uploadResult.Success {
		errMsg := "上传失败"
		if uploadResult != nil && uploadResult.Message != "" {
			errMsg = uploadResult.Message
		}
		output := UploadOutput{
			Success:  false,
			FileName: fileName,
			FileSize: fileSize,
			Error:    errMsg,
			Time:     utils.GetCurrentTimeString(),
		}
		outputJSON, _ := json.Marshal(output)
		item.OutputData = string(outputJSON)
		return fmt.Errorf("上传失败: %s", errMsg)
	}

	// 提取上传结果
	var shareURL string
	if data, ok := uploadResult.Data.(map[string]interface{}); ok {
		if v, ok := data["shareUrl"]; ok {
			shareURL, _ = v.(string)
		}
	}

	// 获取 panID
	panIDInt, _ := up.repoMgr.PanRepository.FindIdByServiceType("quark")
	panID := uint(panIDInt)

	// 保存�?resources �?
	resource := &entity.Resource{
		Title:     fileName,
		URL:       shareURL,
		SaveURL:   shareURL,
		PanID:     &panID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsValid:   true,
		IsPublic:  true,
		CkID:      &input.CkID,
	}

	if err := up.repoMgr.ResourceRepository.Create(resource); err != nil {
		utils.Error("保存资源记录失败: %v", err)
		// 资源保存失败不影响上传结�?
	} else {
		utils.Info("上传结果已保存到资源库，资源ID: %d", resource.ID)
	}

	// 删除本地源文�?
	if err := os.Remove(input.FilePath); err != nil {
		utils.Warn("删除本地源文件失�? %s, 错误: %v", input.FilePath, err)
	} else {
		utils.Info("已删除本地源文件: %s", input.FilePath)
	}

	// 构建成功输出
	output := UploadOutput{
		Success:     true,
		FileName:    fileName,
		FileSize:    fileSize,
		ShareURL:    shareURL,
		ResourceID:  resource.ID,
		IsDuplicate: false,
		Time:        utils.GetCurrentTimeString(),
	}

	outputJSON, _ := json.Marshal(output)
	item.OutputData = string(outputJSON)

	elapsedTime := time.Since(startTime)
	utils.InfoWithFields(map[string]interface{}{
		"task_item_id": item.ID,
		"file_name":    fileName,
		"share_url":    shareURL,
		"resource_id":  resource.ID,
		"duration_ms":  elapsedTime.Milliseconds(),
	}, "上传任务项处理完�? %s, 资源ID: %d, 耗时: %v", fileName, resource.ID, elapsedTime)

	return nil
}
