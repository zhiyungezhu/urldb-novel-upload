?????????package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/task"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"

	"github.com/gin-gonic/gin"
)

// TaskHandler ��������
type TaskHandler struct {
	repoMgr     *repo.RepositoryManager
	taskManager *task.TaskManager
}

// NewTaskHandler ������������
func NewTaskHandler(repoMgr *repo.RepositoryManager, taskManager *task.TaskManager) *TaskHandler {
	return &TaskHandler{
		repoMgr:     repoMgr,
		taskManager: taskManager,
	}
}

// ����ת��������Դ��
type BatchTransferResource struct {
	Title      string `json:"title" binding:"required"`
	URL        string `json:"url" binding:"required"`
	CategoryID uint   `json:"category_id,omitempty"`
	PanID      uint   `json:"pan_id,omitempty"`
	Tags       []uint `json:"tags,omitempty"`
}

// CreateBatchTransferTask ��������ת������
func (h *TaskHandler) CreateBatchTransferTask(c *gin.Context) {
	var req struct {
		Title            string                  `json:"title" binding:"required"`
		Description      string                  `json:"description"`
		Resources        []BatchTransferResource `json:"resources" binding:"required,min=1"`
		SelectedAccounts []uint                  `json:"selected_accounts,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, "��������: "+err.Error(), http.StatusBadRequest)
		return
	}

	username, _ := c.Get("username")
	clientIP, _ := c.Get("client_ip")
	utils.Info("CreateBatchTransferTask - �û���������ת������ - �û�: %s, �������: %s, ��Դ����: %d, IP: %s", username, req.Title, len(req.Resources), clientIP)

	utils.Debug("��������ת������: %s����Դ����: %d��ѡ���˺�����: %d", req.Title, len(req.Resources), len(req.SelectedAccounts))

	// ������������
	taskConfig := map[string]interface{}{
		"selected_accounts": req.SelectedAccounts,
	}
	configJSON, _ := json.Marshal(taskConfig)

	// ��������
	newTask := &entity.Task{
		Title:       req.Title,
		Description: req.Description,
		Type:        "transfer",
		Status:      "pending",
		TotalItems:  len(req.Resources),
		Config:      string(configJSON),
		CreatedAt:   utils.GetCurrentTime(),
		UpdatedAt:   utils.GetCurrentTime(),
	}

	err := h.repoMgr.TaskRepository.Create(newTask)
	if err != nil {
		utils.Error("��������ʧ��: %v", err)
		ErrorResponse(c, "��������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ����������
	for _, resource := range req.Resources {
		// ����ת����������
		transferInput := task.TransferInput{
			Title:      resource.Title,
			URL:        resource.URL,
			CategoryID: resource.CategoryID,
			PanID:      resource.PanID,
			Tags:       resource.Tags,
		}

		inputJSON, _ := json.Marshal(transferInput)

		taskItem := &entity.TaskItem{
			TaskID:    newTask.ID,
			Status:    "pending",
			InputData: string(inputJSON),
			CreatedAt: utils.GetCurrentTime(),
			UpdatedAt: utils.GetCurrentTime(),
		}

		err = h.repoMgr.TaskItemRepository.Create(taskItem)
		if err != nil {
			utils.Error("����������ʧ��: %v", err)
			// ������������������
		}
	}

	utils.Debug("����ת�����񴴽����: %d, �� %d ��", newTask.ID, len(req.Resources))

	SuccessResponse(c, gin.H{
		"task_id":     newTask.ID,
		"total_items": len(req.Resources),
		"message":     "���񴴽��ɹ�",
	})
}

// StartTask ��������
func (h *TaskHandler) StartTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		ErrorResponse(c, "��Ч������ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	username, _ := c.Get("username")
	clientIP, _ := c.Get("client_ip")
	utils.Info("StartTask - �û��������� - �û�: %s, ����ID: %d, IP: %s", username, taskID, clientIP)

	err = h.taskManager.StartTask(uint(taskID))
	if err != nil {
		utils.Error("��������ʧ��: %v", err)
		ErrorResponse(c, "��������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Debug("��������: %d", taskID)

	SuccessResponse(c, gin.H{
		"message": "���������ɹ�",
	})
}

// StopTask ֹͣ����
func (h *TaskHandler) StopTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		ErrorResponse(c, "��Ч������ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	username, _ := c.Get("username")
	clientIP, _ := c.Get("client_ip")
	utils.Info("StopTask - �û�ֹͣ���� - �û�: %s, ����ID: %d, IP: %s", username, taskID, clientIP)

	err = h.taskManager.StopTask(uint(taskID))
	if err != nil {
		utils.Error("ֹͣ����ʧ��: %v", err)
		ErrorResponse(c, "ֹͣ����ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Debug("ֹͣ����: %d", taskID)

	SuccessResponse(c, gin.H{
		"message": "����ֹͣ�ɹ�",
	})
}

// PauseTask ��ͣ����
func (h *TaskHandler) PauseTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		ErrorResponse(c, "��Ч������ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	username, _ := c.Get("username")
	clientIP, _ := c.Get("client_ip")
	utils.Info("PauseTask - �û���ͣ���� - �û�: %s, ����ID: %d, IP: %s", username, taskID, clientIP)

	err = h.taskManager.PauseTask(uint(taskID))
	if err != nil {
		utils.Error("��ͣ����ʧ��: %v", err)
		ErrorResponse(c, "��ͣ����ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Debug("��ͣ����: %d", taskID)

	SuccessResponse(c, gin.H{
		"message": "������ͣ�ɹ�",
	})
}

// GetTaskStatus ��ȡ����״̬
func (h *TaskHandler) GetTaskStatus(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		ErrorResponse(c, "��Ч������ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	// ��ȡ��������
	task, err := h.repoMgr.TaskRepository.GetByID(uint(taskID))
	if err != nil {
		ErrorResponse(c, "���񲻴���: "+err.Error(), http.StatusNotFound)
		return
	}

	// ��ȡ������ͳ��
	stats, err := h.repoMgr.TaskItemRepository.GetStatsByTaskID(uint(taskID))
	if err != nil {
		utils.Error("��ȡ������ͳ��ʧ��: %v", err)
		stats = map[string]int{
			"total":      0,
			"pending":    0,
			"processing": 0,
			"completed":  0,
			"failed":     0,
		}
	}

	// ��������Ƿ�������
	isRunning := h.taskManager.IsTaskRunning(uint(taskID))

	SuccessResponse(c, gin.H{
		"id":              task.ID,
		"title":           task.Title,
		"description":     task.Description,
		"task_type":       task.Type,
		"status":          task.Status,
		"total_items":     task.TotalItems,
		"processed_items": task.ProcessedItems,
		"success_items":   task.SuccessItems,
		"failed_items":    task.FailedItems,
		"is_running":      isRunning,
		"stats":           stats,
		"created_at":      task.CreatedAt,
		"updated_at":      task.UpdatedAt,
	})
}

// GetTasks ��ȡ�����б�
func (h *TaskHandler) GetTasks(c *gin.Context) {
	// ��ȡ��ѯ����
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "10")
	taskType := c.Query("taskType")
	status := c.Query("status")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	utils.Debug("GetTasks: ��ȡ�����б� page=%d, pageSize=%d, taskType=%s, status=%s", page, pageSize, taskType, status)

	// ��ȡ�����б�
	tasks, total, err := h.repoMgr.TaskRepository.GetList(page, pageSize, taskType, status)
	if err != nil {
		utils.Error("��ȡ�����б�ʧ��: %v", err)
		ErrorResponse(c, "��ȡ�����б�ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Debug("GetTasks: �����ݿ��ȡ�� %d ������", len(tasks))

	// ��ȡ��������״̬
	var taskList []gin.H
	for _, task := range tasks {
		isRunning := h.taskManager.IsTaskRunning(task.ID)
		utils.Debug("GetTasks: ���� %d (%s) ���ݿ�״̬: %s, TaskManager����״̬: %v", task.ID, task.Title, task.Status, isRunning)

		taskList = append(taskList, gin.H{
			"id":              task.ID,
			"title":           task.Title,
			"description":     task.Description,
			"type":            task.Type,
			"status":          task.Status,
			"total_items":     task.TotalItems,
			"processed_items": task.ProcessedItems,
			"success_items":   task.SuccessItems,
			"failed_items":    task.FailedItems,
			"is_running":      isRunning,
			"created_at":      task.CreatedAt,
			"updated_at":      task.UpdatedAt,
		})
	}

	SuccessResponse(c, gin.H{
		"tasks":       taskList,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// GetTaskItems ��ȡ�������б�
func (h *TaskHandler) GetTaskItems(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		ErrorResponse(c, "��Ч������ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10000"))
	status := c.Query("status")

	items, total, err := h.repoMgr.TaskItemRepository.GetListByTaskID(uint(taskID), page, pageSize, status)
	if err != nil {
		utils.Error("��ȡ�������б�ʧ��: %v", err)
		ErrorResponse(c, "��ȡ�������б�ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ����������������
	var result []gin.H
	for _, item := range items {
		itemData := gin.H{
			"id":         item.ID,
			"status":     item.Status,
			"created_at": item.CreatedAt,
			"updated_at": item.UpdatedAt,
		}

		// ������������
		if item.InputData != "" {
			var inputData map[string]interface{}
			if err := json.Unmarshal([]byte(item.InputData), &inputData); err == nil {
				itemData["input"] = inputData
			}
		}

		// �����������
		if item.OutputData != "" {
			var outputData map[string]interface{}
			if err := json.Unmarshal([]byte(item.OutputData), &outputData); err == nil {
				itemData["output"] = outputData
			}
		}

		result = append(result, itemData)
	}

	SuccessResponse(c, gin.H{
		"items": result,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// DeleteTask ɾ������
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		ErrorResponse(c, "��Ч������ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	username, _ := c.Get("username")
	clientIP, _ := c.Get("client_ip")
	utils.Info("DeleteTask - �û�ɾ������ - �û�: %s, ����ID: %d, IP: %s", username, taskID, clientIP)

	// ��������Ƿ�������
	if h.taskManager.IsTaskRunning(uint(taskID)) {
		utils.Warn("DeleteTask - ����ɾ���������е����� - �û�: %s, ����ID: %d, IP: %s", username, taskID, clientIP)
		ErrorResponse(c, "�������������У��޷�ɾ��", http.StatusBadRequest)
		return
	}

	// ɾ��������
	err = h.repoMgr.TaskItemRepository.DeleteByTaskID(uint(taskID))
	if err != nil {
		utils.Error("ɾ��������ʧ��: %v", err)
		ErrorResponse(c, "ɾ��������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ɾ������
	err = h.repoMgr.TaskRepository.Delete(uint(taskID))
	if err != nil {
		utils.Error("ɾ������ʧ��: %v", err)
		ErrorResponse(c, "ɾ������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.Debug("����ɾ���ɹ�: %d", taskID)
	utils.Info("DeleteTask - ����ɾ���ɹ� - �û�: %s, ����ID: %d, IP: %s", username, taskID, clientIP)

	SuccessResponse(c, gin.H{
		"message": "����ɾ���ɹ�",
	})
}

// BatchUploadResource �����ϴ�������Դ��
type BatchUploadResource struct {
	Title      string `json:"title" binding:"required"`
	FilePath   string `json:"file_path" binding:"required"`
	PdirFid    string `json:"pdir_fid"`
	CategoryID uint   `json:"category_id,omitempty"`
	PanID      uint   `json:"pan_id,omitempty"`
	Tags       []uint `json:"tags,omitempty"`
}

// CreateUploadTask �����ϴ�����
func (h *TaskHandler) CreateUploadTask(c *gin.Context) {
	var req struct {
		Title            string                `json:"title" binding:"required"`
		Description      string                `json:"description"`
		Resources        []BatchUploadResource `json:"resources" binding:"required,min=1"`
		SelectedAccounts []uint                `json:"selected_accounts,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, "��������: "+err.Error(), http.StatusBadRequest)
		return
	}

	username, _ := c.Get("username")
	clientIP, _ := c.Get("client_ip")
	utils.Info("CreateUploadTask - �û������ϴ����� - �û�: %s, �������: %s, �ļ�����: %d, IP: %s", username, req.Title, len(req.Resources), clientIP)

	// ����˺ţ�ȡ��һ��ѡ�е��˺ţ�
	if len(req.SelectedAccounts) == 0 {
		ErrorResponse(c, "��ѡ���ϴ��˺�", http.StatusBadRequest)
		return
	}

	// ������������
	taskConfig := map[string]interface{}{
		"selected_accounts": req.SelectedAccounts,
	}
	configJSON, _ := json.Marshal(taskConfig)

	// ��������
	newTask := &entity.Task{
		Title:       req.Title,
		Description: req.Description,
		Type:        "upload",
		Status:      "pending",
		TotalItems:  len(req.Resources),
		Config:      string(configJSON),
		CreatedAt:   utils.GetCurrentTime(),
		UpdatedAt:   utils.GetCurrentTime(),
	}

	err := h.repoMgr.TaskRepository.Create(newTask)
	if err != nil {
		utils.Error("�����ϴ�����ʧ��: %v", err)
		ErrorResponse(c, "��������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ʹ�õ�һ��ѡ�е��˺�ID
	ckID := req.SelectedAccounts[0]

	// ����������
	for _, resource := range req.Resources {
		uploadInput := task.UploadInput{
			FilePath: resource.FilePath,
			CkID:     ckID,
			PdirFid:  resource.PdirFid,
		}

		inputJSON, _ := json.Marshal(uploadInput)

		taskItem := &entity.TaskItem{
			TaskID:    newTask.ID,
			Status:    "pending",
			InputData: string(inputJSON),
			CreatedAt: utils.GetCurrentTime(),
			UpdatedAt: utils.GetCurrentTime(),
		}

		err = h.repoMgr.TaskItemRepository.Create(taskItem)
		if err != nil {
			utils.Error("�����ϴ�������ʧ��: %v", err)
		}
	}

	utils.Info("�ϴ����񴴽����: %d, �� %d ���ļ�", newTask.ID, len(req.Resources))

	SuccessResponse(c, gin.H{
		"task_id":     newTask.ID,
		"total_items": len(req.Resources),
		"message":     "�ϴ����񴴽��ɹ�",
	})
}

// CreateExpansionTask ������������
		PanAccountID uint                   `json:"pan_account_id" binding:"required"`
		Description  string                 `json:"description"`
		DataSource   map[string]interface{} `json:"dataSource"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, "��������: "+err.Error(), http.StatusBadRequest)
		return
	}

	username, _ := c.Get("username")
	clientIP, _ := c.Get("client_ip")
	utils.Info("CreateExpansionTask - �û������������� - �û�: %s, �˺�ID: %d, IP: %s", username, req.PanAccountID, clientIP)

	utils.Debug("������������: �˺�ID %d", req.PanAccountID)

	// ��ȡ�˺���Ϣ�����ڹ����������
	cks, err := h.repoMgr.CksRepository.FindByID(req.PanAccountID)
	if err != nil {
		utils.Error("��ȡ�˺���Ϣʧ��: %v", err)
		ErrorResponse(c, "��ȡ�˺���Ϣʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// �����˺�����
	accountName := cks.Username
	if accountName == "" {
		accountName = cks.Remark
	}
	if accountName == "" {
		accountName = fmt.Sprintf("�˺�%d", cks.ID)
	}

	// �����������ã��洢�˺�ID������Դ��
	taskConfig := map[string]interface{}{
		"pan_account_id": req.PanAccountID,
	}
	// ���������Դ���ã����ӵ�taskConfig��
	if req.DataSource != nil && len(req.DataSource) > 0 {
		taskConfig["data_source"] = req.DataSource
	}
	configJSON, _ := json.Marshal(taskConfig)

	// ����������⣬�����˺�����
	taskTitle := fmt.Sprintf("�˺����� - %s", accountName)

	// ��������
	newTask := &entity.Task{
		Title:       taskTitle,
		Description: req.Description,
		Type:        "expansion",
		Status:      "pending",
		TotalItems:  1, // ��������ֻ��һ����Ŀ
		Config:      string(configJSON),
		CreatedAt:   utils.GetCurrentTime(),
		UpdatedAt:   utils.GetCurrentTime(),
	}

	if err := h.repoMgr.TaskRepository.Create(newTask); err != nil {
		utils.Error("������������ʧ��: %v", err)
		ErrorResponse(c, "��������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ����������
	expansionInput := task.ExpansionInput{
		PanAccountID: req.PanAccountID,
	}
	// ���������Դ���ã����ӵ�����������
	if req.DataSource != nil && len(req.DataSource) > 0 {
		expansionInput.DataSource = req.DataSource
	}

	inputJSON, _ := json.Marshal(expansionInput)

	taskItem := &entity.TaskItem{
		TaskID:    newTask.ID,
		Status:    "pending",
		InputData: string(inputJSON),
		CreatedAt: utils.GetCurrentTime(),
		UpdatedAt: utils.GetCurrentTime(),
	}

	err = h.repoMgr.TaskItemRepository.Create(taskItem)
	if err != nil {
		utils.Error("��������������ʧ��: %v", err)
		// ���������������ش���
	}

	utils.Debug("�������񴴽����: %d", newTask.ID)

	SuccessResponse(c, gin.H{
		"task_id":     newTask.ID,
		"total_items": 1,
		"message":     "�������񴴽��ɹ�",
	})
}

// GetExpansionAccounts ��ȡ֧�����ݵ��˺��б�
func (h *TaskHandler) GetExpansionAccounts(c *gin.Context) {
	// ��ȡ������Ч���˺�
	cksList, err := h.repoMgr.CksRepository.FindByIsValid(false)
	if err != nil {
		utils.Error("��ȡ�˺��б�ʧ��: %v", err)
		ErrorResponse(c, "��ȡ�˺��б�ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ���˳� quark �˺�
	var expansionAccounts []gin.H
	tasks, _, _ := h.repoMgr.TaskRepository.GetList(1, 1000, "expansion", "completed")
	for _, ck := range cksList {
		if ck.ServiceType == "quark" {
			// ʹ�� Username ��Ϊ�˺����ƣ����Ϊ����ʹ�� Remark
			accountName := ck.Username
			if accountName == "" {
				accountName = ck.Remark
			}
			if accountName == "" {
				accountName = "�˺� " + fmt.Sprintf("%d", ck.ID)
			}

			// ����Ƿ��Ѿ����ݹ�
			expanded := false
			for _, task := range tasks {
				if task.Config != "" {
					var taskConfig map[string]interface{}
					if err := json.Unmarshal([]byte(task.Config), &taskConfig); err == nil {
						if configAccountID, ok := taskConfig["pan_account_id"].(float64); ok {
							if uint(configAccountID) == ck.ID {
								expanded = true
								break
							}
						}
					}
				}
			}

			expansionAccounts = append(expansionAccounts, gin.H{
				"id":           ck.ID,
				"name":         accountName,
				"service_type": ck.ServiceType,
				"expanded":     expanded,
				"total_space":  ck.Space,
				"used_space":   ck.UsedSpace,
				"created_at":   ck.CreatedAt,
				"updated_at":   ck.UpdatedAt,
			})
		}
	}

	SuccessResponse(c, gin.H{
		"accounts": expansionAccounts,
		"total":    len(expansionAccounts),
		"message":  "��ȡ֧�������˺��б��ɹ�",
	})
}

// GetExpansionOutput ��ȡ�˺������������
func (h *TaskHandler) GetExpansionOutput(c *gin.Context) {
	accountIDStr := c.Param("accountId")
	accountID, err := strconv.ParseUint(accountIDStr, 10, 32)
	if err != nil {
		ErrorResponse(c, "��Ч���˺�ID: "+err.Error(), http.StatusBadRequest)
		return
	}

	utils.Debug("��ȡ�˺������������: �˺�ID %d", accountID)

	// ��ȡ���˺ŵ�������������
	tasks, _, err := h.repoMgr.TaskRepository.GetList(1, 1000, "expansion", "completed")
	if err != nil {
		utils.Error("��ȡ���������б�ʧ��: %v", err)
		ErrorResponse(c, "��ȡ���������б�ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ���Ҹ��˺ŵ���������
	var targetTask *entity.Task
	for _, task := range tasks {
		if task.Config != "" {
			var taskConfig map[string]interface{}
			if err := json.Unmarshal([]byte(task.Config), &taskConfig); err == nil {
				if configAccountID, ok := taskConfig["pan_account_id"].(float64); ok {
					if uint(configAccountID) == uint(accountID) {
						targetTask = task
						break
					}
				}
			}
		}
	}

	if targetTask == nil {
		ErrorResponse(c, "���˺�û�������������", http.StatusNotFound)
		return
	}

	// ��ȡ�������ȡ�������
	items, _, err := h.repoMgr.TaskItemRepository.GetListByTaskID(targetTask.ID, 1, 10, "completed")
	if err != nil {
		utils.Error("��ȡ������ʧ��: %v", err)
		ErrorResponse(c, "��ȡ�����������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(items) == 0 {
		ErrorResponse(c, "���������", http.StatusNotFound)
		return
	}

	// ���ص�һ����ɵ���������������
	taskItem := items[0]
	var outputData map[string]interface{}
	if taskItem.OutputData != "" {
		if err := json.Unmarshal([]byte(taskItem.OutputData), &outputData); err != nil {
			utils.Error("�����������ʧ��: %v", err)
			ErrorResponse(c, "�����������ʧ��: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	SuccessResponse(c, gin.H{
		"task_id":     targetTask.ID,
		"account_id":  accountID,
		"output_data": outputData,
		"message":     "��ȡ����������ݳɹ�",
	})
}

// NovelUploadResource С˵�ϴ���Դ��
type NovelUploadResource struct {
	FolderPath string `json:"folder_path" binding:"required"` // ����С˵�ļ���·��
	NovelName  string `json:"novel_name,omitempty"`           // С˵���ƣ�����ȡ�ļ�������
}

// CreateNovelUploadTask ����С˵�ϴ�����
func (h *TaskHandler) CreateNovelUploadTask(c *gin.Context) {
	var req struct {
		Title            string               `json:"title" binding:"required"`
		Description      string               `json:"description"`
		Resources        []NovelUploadResource `json:"resources" binding:"required,min=1"`
		SelectedAccounts []uint               `json:"selected_accounts,omitempty"`
		ParentFid        string               `json:"parent_fid,omitempty"` // ��˸�Ŀ¼ID
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, "��������: "+err.Error(), http.StatusBadRequest)
		return
	}

	username, _ := c.Get("username")
	utils.Info("CreateNovelUploadTask - �û�����С˵�ϴ����� - �û�: %s, ����: %s, ����: %d", username, req.Title, len(req.Resources))

	if len(req.SelectedAccounts) == 0 {
		ErrorResponse(c, "������ѡ��һ���˺�", http.StatusBadRequest)
		return
	}

	ckID := req.SelectedAccounts[0]
	parentFid := req.ParentFid
	if parentFid == "" {
		parentFid = "0"
	}

	now := time.Now()
	config := map[string]interface{}{
		"selected_accounts": req.SelectedAccounts,
		"parent_fid":        parentFid,
	}
	configJSON, _ := json.Marshal(config)

	newTask := &entity.Task{
		Title:       req.Title,
		Description: req.Description,
		Type:        "novel_upload",
		Status:      "pending",
		TotalItems:  len(req.Resources),
		Config:      string(configJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.repoMgr.TaskRepository.Create(newTask); err != nil {
		utils.Error("����С˵�ϴ�����ʧ��: %v", err)
		ErrorResponse(c, "��������ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ϊÿ��С˵�ļ��д���������
	successCount := 0
	for _, resource := range req.Resources {
		folderPath := resource.FolderPath
		novelName := resource.NovelName
		if novelName == "" {
			novelName = filepath.Base(folderPath)
		}

		input := task.NovelUploadInput{
			FolderPath: folderPath,
			NovelName:  novelName,
			CkID:       ckID,
			ParentFid:  parentFid,
		}
		inputJSON, _ := json.Marshal(input)

		taskItem := &entity.TaskItem{
			TaskID:    newTask.ID,
			Status:    "pending",
			InputData: string(inputJSON),
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := h.repoMgr.TaskItemRepository.Create(taskItem); err != nil {
			utils.Error("����С˵�ϴ�������ʧ��: %s, ����: %v", novelName, err)
			continue
		}
		successCount++
	}

	utils.Info("С˵�ϴ����񴴽����: ����ID=%d, ����=%d/%d", newTask.ID, successCount, len(req.Resources))
	SuccessResponse(c, gin.H{
		"task_id":   newTask.ID,
		"count":     successCount,
		"total":     len(req.Resources),
		"message":   "С˵�ϴ����񴴽��ɹ�",
	})
}
