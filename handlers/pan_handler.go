package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	panutils "github.com/zhiyungezhu/urldb-novel-upload/common"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// BrowsePanFolders �������ָ��Ŀ¼�µ��ļ���
// GET /api/pan/quark/folders?ck_id=1&pdir_fid=0
// ���ص�ǰĿ¼�µ����ļ����б��������ļ���
func BrowsePanFolders(c *gin.Context) {
	ckIDStr := c.Query("ck_id")
	pdirFid := c.DefaultQuery("pdir_fid", "0")

	if ckIDStr == "" {
		ErrorResponse(c, "���ṩ�˺�ID (ck_id)", http.StatusBadRequest)
		return
	}

	ckID, err := strconv.ParseUint(ckIDStr, 10, 64)
	if err != nil {
		ErrorResponse(c, "��Ч���˺�ID", http.StatusBadRequest)
		return
	}

	clientIP, _ := c.Get("client_ip")
	utils.Info("BrowsePanFolders - IP: %s, ck_id: %d, pdir_fid: %s", clientIP, ckID, pdirFid)

	// ��ȡ�˺���Ϣ
	cks, err := repoManager.CksRepository.FindByID(uint(ckID))
	if err != nil {
		ErrorResponse(c, "��ȡ�˺���Ϣʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if cks.Ck == "" {
		ErrorResponse(c, "���˺�δ����Cookie", http.StatusBadRequest)
		return
	}

	if cks.ServiceType != "quark" {
		ErrorResponse(c, "��ǰ��֧�ֿ������", http.StatusBadRequest)
		return
	}

	// �������̷���
	factory := panutils.NewPanFactory()
	service, err := factory.CreatePanServiceByType(panutils.Quark, &panutils.PanConfig{
		Cookie: cks.Ck,
	})
	if err != nil {
		ErrorResponse(c, "�������̷���ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}
	service.SetCKSRepository(repoManager.CksRepository, *cks)

	// ��ȡ�ļ��б�
	result, err := service.GetFiles(pdirFid)
	if err != nil {
		ErrorResponse(c, "��ȡĿ¼�б�ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if result == nil || !result.Success {
		msg := "��ȡĿ¼�б�ʧ��"
		if result != nil {
			msg = result.Message
		}
		ErrorResponse(c, msg, http.StatusInternalServerError)
		return
	}

	// ���˳��ļ��У�dir=true �� obj_category="dir"��
	fileList, ok := result.Data.([]interface{})
	if !ok {
		ErrorResponse(c, "�������ݸ�ʽ�쳣", http.StatusInternalServerError)
		return
	}

	type FolderInfo struct {
		Fid      string `json:"fid"`
		FileName string `json:"file_name"`
	}

	folders := make([]FolderInfo, 0)
	for _, item := range fileList {
		if fileMap, ok := item.(map[string]interface{}); ok {
			isDir := false
			if dirVal, ok := fileMap["dir"].(bool); ok && dirVal {
				isDir = true
			}
			if cat, ok := fileMap["obj_category"].(string); ok && cat == "dir" {
				isDir = true
			}
			if ft, ok := fileMap["file_type"].(string); ok && ft == "folder" {
				isDir = true
			}

			if isDir {
				fid, _ := fileMap["fid"].(string)
				fileName, _ := fileMap["file_name"].(string)
				folders = append(folders, FolderInfo{
					Fid:      fid,
					FileName: fileName,
				})
			}
		}
	}

	SuccessResponse(c, gin.H{
		"pdir_fid": pdirFid,
		"folders":  folders,
		"count":    len(folders),
	})
}

// CheckPanCookie �������Cookie�Ƿ���Ч
// GET /api/pan/quark/check-cookie?ck_id=1
func CheckPanCookie(c *gin.Context) {
	ckIDStr := c.Query("ck_id")
	if ckIDStr == "" {
		ErrorResponse(c, "���ṩ�˺�ID (ck_id)", http.StatusBadRequest)
		return
	}

	ckID, err := strconv.ParseUint(ckIDStr, 10, 64)
	if err != nil {
		ErrorResponse(c, "��Ч���˺�ID", http.StatusBadRequest)
		return
	}

	// ��ȡ�˺���Ϣ
	cks, err := repoManager.CksRepository.FindByID(uint(ckID))
	if err != nil {
		ErrorResponse(c, "��ȡ�˺���Ϣʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if cks.Ck == "" {
		ErrorResponse(c, "���˺�δ����Cookie", http.StatusBadRequest)
		return
	}

	// �������̷���
	factory := panutils.NewPanFactory()
	service, err := factory.CreatePanServiceByType(panutils.Quark, &panutils.PanConfig{
		Cookie: cks.Ck,
	})
	if err != nil {
		ErrorResponse(c, "�������̷���ʧ��: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// ���� GetUserInfo У�� Cookie
	userInfo, err := service.GetUserInfo(&cks.Ck)
	if err != nil {
		ErrorResponse(c, "Cookie��֤ʧ�ܣ������ѹ���: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// �����˺���Ϣ
	if userInfo != nil {
		cks.Username = userInfo.Username
		cks.Space = userInfo.TotalSpace
		cks.UsedSpace = userInfo.UsedSpace
		cks.LeftSpace = userInfo.TotalSpace - userInfo.UsedSpace
		_ = repoManager.CksRepository.Update(cks)
	}

	SuccessResponse(c, gin.H{
		"valid":       true,
		"ck_id":       ckID,
		"username":    cks.Username,
		"total_space": cks.Space,
		"used_space":  cks.UsedSpace,
		"left_space":  cks.LeftSpace,
	})
}
