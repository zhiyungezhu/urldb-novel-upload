?????package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	panutils "github.com/zhiyungezhu/urldb-novel-upload/common"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// BrowsePanFolders 浏览网盘指定目录下的文件夹
// GET /api/pan/quark/folders?ck_id=1&pdir_fid=0
// 返回当前目录下的子文件夹列表（不含文件）
func BrowsePanFolders(c *gin.Context) {
	ckIDStr := c.Query("ck_id")
	pdirFid := c.DefaultQuery("pdir_fid", "0")

	if ckIDStr == "" {
		ErrorResponse(c, "请提供账号ID (ck_id)", http.StatusBadRequest)
		return
	}

	ckID, err := strconv.ParseUint(ckIDStr, 10, 64)
	if err != nil {
		ErrorResponse(c, "无效的账号ID", http.StatusBadRequest)
		return
	}

	clientIP, _ := c.Get("client_ip")
	utils.Info("BrowsePanFolders - IP: %s, ck_id: %d, pdir_fid: %s", clientIP, ckID, pdirFid)

	// 获取账号信息
	cks, err := repoManager.CksRepository.FindByID(uint(ckID))
	if err != nil {
		ErrorResponse(c, "获取账号信息失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if cks.Ck == "" {
		ErrorResponse(c, "该账号未配置Cookie", http.StatusBadRequest)
		return
	}

	if cks.ServiceType != "quark" {
		ErrorResponse(c, "当前仅支持夸克网盘", http.StatusBadRequest)
		return
	}

	// 创建网盘服务
	factory := panutils.NewPanFactory()
	service, err := factory.CreatePanServiceByType(panutils.Quark, &panutils.PanConfig{
		Cookie: cks.Ck,
	})
	if err != nil {
		ErrorResponse(c, "创建网盘服务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	service.SetCKSRepository(repoManager.CksRepository, *cks)

	// 获取文件列表
	result, err := service.GetFiles(pdirFid)
	if err != nil {
		ErrorResponse(c, "获取目录列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if result == nil || !result.Success {
		msg := "获取目录列表失败"
		if result != nil {
			msg = result.Message
		}
		ErrorResponse(c, msg, http.StatusInternalServerError)
		return
	}

	// 过滤出文件夹（dir=true 或 obj_category="dir"）
	fileList, ok := result.Data.([]interface{})
	if !ok {
		ErrorResponse(c, "返回数据格式异常", http.StatusInternalServerError)
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

// CheckPanCookie 检查网盘Cookie是否有效
// GET /api/pan/quark/check-cookie?ck_id=1
func CheckPanCookie(c *gin.Context) {
	ckIDStr := c.Query("ck_id")
	if ckIDStr == "" {
		ErrorResponse(c, "请提供账号ID (ck_id)", http.StatusBadRequest)
		return
	}

	ckID, err := strconv.ParseUint(ckIDStr, 10, 64)
	if err != nil {
		ErrorResponse(c, "无效的账号ID", http.StatusBadRequest)
		return
	}

	// 获取账号信息
	cks, err := repoManager.CksRepository.FindByID(uint(ckID))
	if err != nil {
		ErrorResponse(c, "获取账号信息失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if cks.Ck == "" {
		ErrorResponse(c, "该账号未配置Cookie", http.StatusBadRequest)
		return
	}

	// 创建网盘服务
	factory := panutils.NewPanFactory()
	service, err := factory.CreatePanServiceByType(panutils.Quark, &panutils.PanConfig{
		Cookie: cks.Ck,
	})
	if err != nil {
		ErrorResponse(c, "创建网盘服务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 调用 GetUserInfo 校验 Cookie
	userInfo, err := service.GetUserInfo(&cks.Ck)
	if err != nil {
		ErrorResponse(c, "Cookie验证失败，可能已过期: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// 更新账号信息
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
