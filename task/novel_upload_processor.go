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

// NovelUploadProcessor handles uploading a novel folder (containing .txt/.epub/.pdf/bulk_txt) to Quark Pan.
// Steps: 1. Create folder on Quark → 2. Upload all files → 3. Share folder → 4. Delete local files after 12h
type NovelUploadProcessor struct {
	repoMgr *repo.RepositoryManager
}

// NewNovelUploadProcessor creates a new novel upload processor
func NewNovelUploadProcessor(repoMgr *repo.RepositoryManager) *NovelUploadProcessor {
	return &NovelUploadProcessor{
		repoMgr: repoMgr,
	}
}

// GetTaskType returns the task type
func (np *NovelUploadProcessor) GetTaskType() string {
	return "novel_upload"
}

// NovelUploadInput represents the input for a novel upload task
type NovelUploadInput struct {
	FolderPath string `json:"folder_path"` // local novel folder path, e.g. ./output/doupocangqiong/
	NovelName  string `json:"novel_name"`  // novel name (for Quark folder and share title)
	CkID       uint   `json:"ck_id"`       // Quark account ID
	ParentFid  string `json:"parent_fid"`  // Quark parent directory ID (user-created novel category directory)
}

// NovelUploadOutput represents the output of a novel upload task
type NovelUploadOutput struct {
	Success       bool   `json:"success"`
	NovelName     string `json:"novel_name"`
	FolderFid     string `json:"folder_fid,omitempty"`
	FileCount     int    `json:"file_count"`
	UploadedCount int    `json:"uploaded_count"`
	ShareURL      string `json:"share_url,omitempty"`
	ShareCode     string `json:"share_code,omitempty"`
	ResourceID    uint   `json:"resource_id,omitempty"`
	Error         string `json:"error,omitempty"`
	Time          string `json:"time"`
}

// Process handles novel upload tasks
func (np *NovelUploadProcessor) Process(ctx context.Context, taskID uint, item *entity.TaskItem) error {
	startTime := time.Now()
	utils.InfoWithFields(map[string]interface{}{
		"task_item_id": item.ID,
		"task_id":      taskID,
	}, "Start processing novel upload task item: %d", item.ID)

	// Parse input
	var input NovelUploadInput
	if err := json.Unmarshal([]byte(item.InputData), &input); err != nil {
		utils.Error("Failed to parse novel upload input: %v", err)
		return fmt.Errorf("failed to parse novel upload input: %v", err)
	}

	if input.FolderPath == "" {
		return fmt.Errorf("folder path cannot be empty")
	}
	if input.NovelName == "" {
		input.NovelName = filepath.Base(input.FolderPath)
	}
	if input.CkID == 0 {
		return fmt.Errorf("account ID cannot be empty")
	}
	if input.ParentFid == "" {
		input.ParentFid = "0"
	}

	utils.Info("Novel: %s, local path: %s, parent dir: %s", input.NovelName, input.FolderPath, input.ParentFid)

	// Check if local folder exists
	dirInfo, err := os.Stat(input.FolderPath)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("local folder not found: %v", err))
	}
	if !dirInfo.IsDir() {
		return np.fail(item, input, "path is not a directory")
	}

	// Collect all files in the folder (recursive, supports bulk_txt subdirectory)
	var files []string
	filepath.Walk(input.FolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})

	if len(files) == 0 {
		return np.fail(item, input, "no files found in folder")
	}
	utils.Info("Found %d files to upload", len(files))

	// Get account info
	cks, err := np.repoMgr.CksRepository.FindByID(input.CkID)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("failed to get account info: %v", err))
	}

	// Create pan service
	factory := pan.NewPanFactory()
	service, err := factory.CreatePanServiceByType(pan.Quark, &pan.PanConfig{
		Cookie: cks.Ck,
	})
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("failed to create pan service: %v", err))
	}
	service.SetCKSRepository(np.repoMgr.CksRepository, *cks)

	// Verify cookie
	utils.Info("Verifying account cookie...")
	userInfo, err := service.GetUserInfo(&cks.Ck)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("cookie verification failed (may have expired): %v", err))
	}
	utils.Info("Cookie valid, username: %s", userInfo.Username)

	// 1. Create novel folder on Quark Pan
	utils.Info("Creating folder on Quark: %s", input.NovelName)
	novelFolderFid, err := service.Mkdir(input.ParentFid, input.NovelName)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("failed to create Quark folder: %v", err))
	}
	utils.Info("Folder created successfully, fid: %s", novelFolderFid)

	// 2. Upload files one by one to the folder
	uploadedCount := 0
	for _, filePath := range files {
		select {
		case <-ctx.Done():
			utils.Warn("Task cancelled, uploaded %d/%d files", uploadedCount, len(files))
			return ctx.Err()
		default:
		}

		fileName := filepath.Base(filePath)
		utils.Info("Uploading file %d/%d: %s", uploadedCount+1, len(files), fileName)

		uploadResult, err := service.UploadFile(filePath, novelFolderFid)
		if err != nil {
			utils.Error("File upload failed: %s, error: %v", fileName, err)
			continue
		}
		if uploadResult == nil || !uploadResult.Success {
			errMsg := "upload returned failure"
			if uploadResult != nil {
				errMsg = uploadResult.Message
			}
			utils.Error("File upload returned failure: %s, msg: %s", fileName, errMsg)
			continue
		}
		uploadedCount++
	}

	if uploadedCount == 0 {
		return np.fail(item, input, "all files failed to upload")
	}
	utils.Info("File upload complete: %d/%d succeeded", uploadedCount, len(files))

	// 3. Share folder
	utils.Info("Sharing folder: %s", input.NovelName)
	passwordResult, err := service.ShareFolder(novelFolderFid, input.NovelName)
	if err != nil {
		return np.fail(item, input, fmt.Sprintf("failed to share folder: %v", err))
	}

	shareURL := passwordResult.ShareURL
	shareCode := passwordResult.Code
	utils.Info("Folder shared successfully: %s -> %s (code: %s)", input.NovelName, shareURL, shareCode)

	// Get panID
	panIDInt, _ := np.repoMgr.PanRepository.FindIdByServiceType("quark")
	panID := uint(panIDInt)

	// 4. Save to resources table
	resource := &entity.Resource{
		Title:     fmt.Sprintf("[Novel] %s (%d files)", input.NovelName, uploadedCount),
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
		utils.Error("Failed to save resource record: %v", err)
	} else {
		utils.Info("Resource saved, ID: %d", resource.ID)
	}

	// 5. Schedule local folder deletion after 12 hours
	np.scheduleDelayedCleanup(input.FolderPath, 12*time.Hour)

	// Build success output
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
	}, "Novel upload complete: %s, succeeded %d/%d, duration: %v", input.NovelName, uploadedCount, len(files), elapsed)

	return nil
}

// fail quickly builds failure output and returns an error
func (np *NovelUploadProcessor) fail(item *entity.TaskItem, input NovelUploadInput, errMsg string) error {
	output := NovelUploadOutput{
		Success:   false,
		NovelName: input.NovelName,
		Error:     errMsg,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
	}
	outputJSON, _ := json.Marshal(output)
	item.OutputData = string(outputJSON)
	utils.Error("Novel upload failed [%s]: %s", input.NovelName, errMsg)
	return fmt.Errorf(errMsg)
}

// scheduleDelayedCleanup deletes the local folder after a delay
func (np *NovelUploadProcessor) scheduleDelayedCleanup(folderPath string, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		absPath, _ := filepath.Abs(folderPath)
		if err := os.RemoveAll(absPath); err != nil {
			utils.Warn("Delayed cleanup failed: %s, error: %v", absPath, err)
		} else {
			utils.Info("Local folder cleaned up: %s (after %.0f hours)", absPath, delay.Hours())
		}
	}()
}
