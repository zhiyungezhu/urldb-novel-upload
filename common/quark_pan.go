??????package pan

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	commonutils "github.com/zhiyungezhu/urldb-novel-upload/common/utils"
	"github.com/zhiyungezhu/urldb-novel-upload/db"
	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// QuarkPanService 夸克网盘服务
type QuarkPanService struct {
	*BasePanService
	configMutex sync.RWMutex // 保护配置的读写锁
}

// 全局配置缓存刷新信号
var configRefreshChan = make(chan bool, 1)

// 单例相关变量
var (
	systemConfigRepo repo.SystemConfigRepository
	systemConfigOnce sync.Once
)

// NewQuarkPanService 创建夸克网盘服务（单例模式）
func NewQuarkPanService(config *PanConfig) *QuarkPanService {
	quarkInstance := &QuarkPanService{
		BasePanService: NewBasePanService(config),
	}

	// 设置夸克网盘的默认请求头
	quarkInstance.SetHeaders(map[string]string{
		"Accept":             "application/json, text/plain, */*",
		"Accept-Language":    "zh-CN,zh;q=0.9",
		"Content-Type":       "application/json;charset=UTF-8",
		"Sec-Ch-Ua":          `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`,
		"Sec-Ch-Ua-Mobile":   "?0",
		"Sec-Ch-Ua-Platform": `"Windows"`,
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "same-site",
		"Referer":            "https://pan.quark.cn/",
		"Referrer-Policy":    "strict-origin-when-cross-origin",
		"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Cookie":             config.Cookie,
	})

	// 更新配置
	quarkInstance.UpdateConfig(config)

	return quarkInstance
}

// GetQuarkInstance 获取夸克网盘服务单例实例
func GetQuarkInstance() *QuarkPanService {
	return NewQuarkPanService(nil)
}

// UpdateConfig 更新配置（线程安全）
func (q *QuarkPanService) UpdateConfig(config *PanConfig) {
	if config == nil {
		return
	}

	q.configMutex.Lock()
	defer q.configMutex.Unlock()

	q.config = config
	// 设置Cookie到header
	if config.Cookie != "" {
		q.SetHeader("Cookie", config.Cookie)
	}
}

// SetCookie 设置Cookie
func (q *QuarkPanService) SetCookie(cookie string) {
	q.SetHeader("Cookie", cookie)
	q.configMutex.Lock()
	if q.config != nil {
		q.config.Cookie = cookie
	}
	q.configMutex.Unlock()
}

// GetCookie 获取当前Cookie
func (q *QuarkPanService) GetCookie() string {
	return q.GetHeader("Cookie")
}

// GetServiceType 获取服务类型
func (q *QuarkPanService) GetServiceType() ServiceType {
	return Quark
}

// Transfer 转存分享链接
func (q *QuarkPanService) Transfer(shareID string) (*TransferResult, error) {
	// 读取配置（线程安全）
	q.configMutex.RLock()
	config := q.config
	q.configMutex.RUnlock()

	log.Printf("开始处理夸克分享: %s", shareID)

	// 获取stoken
	var stoken string
	if config.Stoken == "" {
		stokenResult, err := q.getStoken(shareID)
		if err != nil {
			return ErrorResult(fmt.Sprintf("获取stoken失败: %v", err)), nil
		}

		stoken = strings.ReplaceAll(stokenResult.Stoken, " ", "+")
	} else {
		stoken = strings.ReplaceAll(config.Stoken, " ", "+")
	}

	// 获取分享详情
	shareResult, err := q.getShare(shareID, stoken)
	if err != nil || len(shareResult.List) == 0 {
		return ErrorResult(fmt.Sprintf("获取分享详情失败: %v", err)), nil
	}

	if config.IsType == 1 {
		// 遍历 资源目录结构
		for _, item := range shareResult.List {
			// 获取文件信息
			fileList, err := q.getDirFile(item.Fid)
			if err != nil {
				log.Printf("获取目录文件失败: %v", err)
				continue
			}

			// 处理文件列表
			if fileList != nil {
				log.Printf("目录 %s 包含 %d 个文件/文件夹", item.Fid, len(fileList))

				// 遍历所有文件，可以在这里添加具体的处理逻辑
				for _, file := range fileList {
					if fileName, ok := file["file_name"].(string); ok {
						if fileType, ok := file["file_type"].(float64); ok {
							fileTypeStr := "文件"
							if fileType == 1 {
								fileTypeStr = "目录"
							}
							log.Printf("  - %s (%s)", fileName, fileTypeStr)
						}
					}
				}
			}
		}

		// 直接返回资源信息
		return SuccessResult("检验成功", map[string]interface{}{
			"title":    shareResult.Share.Title,
			"shareUrl": config.URL,
		}), nil
	}

	// 提取文件信息
	fidList := make([]string, 0)
	fidTokenList := make([]string, 0)
	title := shareResult.Share.Title

	for _, item := range shareResult.List {
		fidList = append(fidList, item.Fid)
		fidTokenList = append(fidTokenList, item.ShareFidToken)
	}

	// 转存资源
	saveResult, err := q.getShareSave(shareID, stoken, fidList, fidTokenList)
	if err != nil {
		return ErrorResult(fmt.Sprintf("转存失败: %v", err)), nil
	}

	taskID := saveResult.TaskID

	// 等待转存完成
	myData, err := q.waitForTask(taskID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("等待转存完成失败: %v", err)), nil
	}

	// 删除广告文件（如果有配置）
	if err := q.deleteAdFiles(myData.SaveAs.SaveAsTopFids[0]); err != nil {
		log.Printf("删除广告文件失败: %v", err)
	}

	// 添加个人自定义广告
	if err := q.addAd(myData.SaveAs.SaveAsTopFids[0]); err != nil {
		log.Printf("添加广告文件失败: %v", err)
	}

	// 分享资源
	shareBtnResult, err := q.getShareBtn(myData.SaveAs.SaveAsTopFids, title)
	if err != nil {
		return ErrorResult(fmt.Sprintf("分享失败: %v", err)), nil
	}

	// 等待分享完成
	shareTaskResult, err := q.waitForTask(shareBtnResult.TaskID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("等待分享完成失败: %v", err)), nil
	}

	// 获取分享密码
	passwordResult, err := q.getSharePassword(shareTaskResult.ShareID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("获取分享密码失败: %v", err)), nil
	}

	// 确定fid
	var fid string
	if len(myData.SaveAs.SaveAsTopFids) > 1 {
		fid = strings.Join(myData.SaveAs.SaveAsTopFids, ",")
	} else {
		fid = passwordResult.FirstFile.Fid
	}

	return SuccessResult("转存成功", map[string]interface{}{
		"shareUrl": passwordResult.ShareURL,
		"title":    passwordResult.ShareTitle,
		"fid":      fid,
		"code":     passwordResult.Code,
	}), nil
}

// GetFiles 获取文件列表
func (q *QuarkPanService) GetFiles(pdirFid string) (*TransferResult, error) {
	if pdirFid == "" {
		pdirFid = "0"
	}

	queryParams := map[string]string{
		"pr":              "ucpro",
		"fr":              "pc",
		"uc_param_str":    "",
		"pdir_fid":        pdirFid,
		"_page":           "1",
		"_size":           "50",
		"_fetch_total":    "1",
		"_fetch_sub_dirs": "0",
		"_sort":           "file_type:asc,updated_at:desc",
	}

	data, err := q.HTTPGet("https://drive-pc.quark.cn/1/clouddrive/file/sort", queryParams)
	if err != nil {
		return ErrorResult(fmt.Sprintf("获取文件列表失败: %v", err)), nil
	}

	var response struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			List []interface{} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return ErrorResult("解析响应失败"), nil
	}

	if response.Status != 200 {
		message := response.Message
		if message == "require login [guest]" {
			message = "夸克未登录，请检查cookie"
		}
		return ErrorResult(message), nil
	}

	return SuccessResult("获取成功", response.Data.List), nil
}

// DeleteFiles 删除文件
func (q *QuarkPanService) DeleteFiles(fileList []string) (*TransferResult, error) {
	if len(fileList) == 0 {
		return ErrorResult("文件列表为空"), nil
	}

	// 逐个删除文件，确保每个删除操作都完成
	for _, fileID := range fileList {
		err := q.deleteSingleFile(fileID)
		if err != nil {
			log.Printf("删除文件 %s 失败: %v", fileID, err)
			return ErrorResult(fmt.Sprintf("删除文件 %s 失败: %v", fileID, err)), nil
		}
	}

	return SuccessResult("删除成功", nil), nil
}

// deleteSingleFile 删除单个文件
func (q *QuarkPanService) deleteSingleFile(fileID string) error {
	log.Printf("正在删除文件: %s", fileID)

	data := map[string]interface{}{
		"action_type":  2,
		"filelist":     []string{fileID},
		"exclude_fids": []string{},
	}

	queryParams := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
	}

	respData, err := q.HTTPPost("https://drive-pc.quark.cn/1/clouddrive/file/delete", data, queryParams)
	if err != nil {
		return fmt.Errorf("删除文件请求失败: %v", err)
	}

	// 解析响应
	var response struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return fmt.Errorf("解析删除响应失败: %v", err)
	}

	if response.Status != 200 {
		return fmt.Errorf("删除文件失败: %s", response.Message)
	}

	// 如果有任务ID，等待任务完成
	if response.Data.TaskID != "" {
		log.Printf("删除文件任务ID: %s", response.Data.TaskID)
		_, err := q.waitForTask(response.Data.TaskID)
		if err != nil {
			return fmt.Errorf("等待删除任务完成失败: %v", err)
		}
		log.Printf("文件 %s 删除完成", fileID)
	} else {
		log.Printf("文件 %s 删除完成（无任务ID）", fileID)
	}

	return nil
}

// getStoken 获取stoken
func (q *QuarkPanService) getStoken(shareID string) (*StokenResult, error) {
	data := map[string]interface{}{
		"passcode": "",
		"pwd_id":   shareID,
	}

	queryParams := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
	}

	respData, err := q.HTTPPost("https://drive-pc.quark.cn/1/clouddrive/share/sharepage/token", data, queryParams)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status  int          `json:"status"`
		Message string       `json:"message"`
		Data    StokenResult `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, err
	}

	if response.Status != 200 {
		return nil, fmt.Errorf(response.Message)
	}

	return &response.Data, nil
}

// getShare 获取分享详情
func (q *QuarkPanService) getShare(shareID, stoken string) (*ShareResult, error) {
	queryParams := map[string]string{
		"pr":            "ucpro",
		"fr":            "pc",
		"uc_param_str":  "",
		"pwd_id":        shareID,
		"stoken":        stoken,
		"pdir_fid":      "0",
		"force":         "0",
		"_page":         "1",
		"_size":         "100",
		"_fetch_banner": "1",
		"_fetch_share":  "1",
		"_fetch_total":  "1",
		"_sort":         "file_type:asc,updated_at:desc",
	}

	respData, err := q.HTTPGet("https://drive-pc.quark.cn/1/clouddrive/share/sharepage/detail", queryParams)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status  int         `json:"status"`
		Message string      `json:"message"`
		Data    ShareResult `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, err
	}

	if response.Status != 200 {
		return nil, fmt.Errorf(response.Message)
	}

	return &response.Data, nil
}

// getShareSave 转存分享
func (q *QuarkPanService) getShareSave(shareID, stoken string, fidList, fidTokenList []string) (*SaveResult, error) {
	return q.getShareSaveToDir(shareID, stoken, fidList, fidTokenList, "0")
}

// getShareSaveToDir 转存分享到指定目录
func (q *QuarkPanService) getShareSaveToDir(shareID, stoken string, fidList, fidTokenList []string, toPdirFid string) (*SaveResult, error) {
	data := map[string]interface{}{
		"pwd_id":         shareID,
		"stoken":         stoken,
		"fid_list":       fidList,
		"fid_token_list": fidTokenList,
		"to_pdir_fid":    toPdirFid, // 存储到指定目录
	}

	queryParams := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
	}

	respData, err := q.HTTPPost("https://drive-pc.quark.cn/1/clouddrive/share/sharepage/save", data, queryParams)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status  int        `json:"status"`
		Message string     `json:"message"`
		Data    SaveResult `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, err
	}

	if response.Status != 200 {
		return nil, fmt.Errorf(response.Message)
	}

	return &response.Data, nil
}

// 生成指定长度的时间戳
func (q *QuarkPanService) generateTimestamp(length int) int64 {
	timestamp := utils.GetCurrentTime().UnixNano() / int64(time.Millisecond)
	timestampStr := strconv.FormatInt(timestamp, 10)
	if len(timestampStr) > length {
		timestampStr = timestampStr[:length]
	}
	timestamp, _ = strconv.ParseInt(timestampStr, 10, 64)
	return timestamp
}

// getShareBtn 分享按钮
func (q *QuarkPanService) getShareBtn(fidList []string, title string) (*ShareBtnResult, error) {
	data := map[string]interface{}{
		"fid_list":     fidList,
		"title":        title,
		"url_type":     1,
		"expired_type": 1, // 永久分享
	}

	queryParams := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
	}

	respData, err := q.HTTPPost("https://drive-pc.quark.cn/1/clouddrive/share", data, queryParams)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status  int            `json:"status"`
		Message string         `json:"message"`
		Data    ShareBtnResult `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, err
	}

	if response.Status != 200 {
		return nil, fmt.Errorf(response.Message)
	}

	return &response.Data, nil
}

// getShareTask 获取分享任务状态
func (q *QuarkPanService) getShareTask(taskID string, retryIndex int) (*TaskResult, error) {
	queryParams := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
		"task_id":      taskID,
		"retry_index":  fmt.Sprintf("%d", retryIndex),
		"__dt":         "21192",
		"__t":          fmt.Sprintf("%d", q.generateTimestamp(13)),
	}

	respData, err := q.HTTPGet("https://drive-pc.quark.cn/1/clouddrive/task", queryParams)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status  int        `json:"status"`
		Message string     `json:"message"`
		Data    TaskResult `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, err
	}

	if response.Status != 200 {
		return nil, fmt.Errorf(response.Message)
	}

	return &response.Data, nil
}

// getSharePassword 获取分享密码
func (q *QuarkPanService) getSharePassword(shareID string) (*PasswordResult, error) {
	queryParams := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
	}

	data := map[string]interface{}{
		"share_id": shareID,
	}

	respData, err := q.HTTPPost("https://drive-pc.quark.cn/1/clouddrive/share/password", data, queryParams)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status  int            `json:"status"`
		Message string         `json:"message"`
		Data    PasswordResult `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, err
	}

	if response.Status != 200 {
		return nil, fmt.Errorf(response.Message)
	}

	return &response.Data, nil
}

// waitForTask 等待任务完成
func (q *QuarkPanService) waitForTask(taskID string) (*TaskResult, error) {
	maxRetries := 50
	retryDelay := 2 * time.Second

	for retryIndex := 0; retryIndex < maxRetries; retryIndex++ {
		result, err := q.getShareTask(taskID, retryIndex)
		if err != nil {
			if strings.Contains(err.Error(), "capacity limit[{0}]") {
				return nil, fmt.Errorf("容量不足")
			}
			return nil, err
		}

		if result.Status == 2 { // 任务完成
			return result, nil
		}

		time.Sleep(retryDelay)
	}

	return nil, fmt.Errorf("任务超时")
}

// deleteAdFiles 删除广告文件
func (q *QuarkPanService) deleteAdFiles(pdirFid string) error {
	log.Printf("开始删除广告文件，目录ID: %s", pdirFid)

	// 获取目录文件列表
	fileList, err := q.getDirFile(pdirFid)
	if err != nil {
		log.Printf("获取目录文件失败: %v", err)
		return err
	}

	if fileList == nil || len(fileList) == 0 {
		log.Printf("目录为空，无需删除广告文件")
		return nil
	}

	// 删除包含广告关键词的文件
	for _, file := range fileList {
		if fileName, ok := file["file_name"].(string); ok {
			log.Printf("检查文件: %s", fileName)
			if q.containsAdKeywords(fileName) {
				if fid, ok := file["fid"].(string); ok {
					log.Printf("删除广告文件: %s (FID: %s)", fileName, fid)
					_, err := q.DeleteFiles([]string{fid})
					if err != nil {
						log.Printf("删除广告文件失败: %v", err)
					} else {
						log.Printf("成功删除广告文件: %s", fileName)
					}
				}
			}
		}
	}

	return nil
}

// containsAdKeywords 检查文件名是否包含广告关键词
func (q *QuarkPanService) containsAdKeywords(filename string) bool {
	// 从系统配置中获取广告关键词
	adKeywordsStr, err := q.getSystemConfigValue(entity.ConfigKeyAdKeywords)
	if err != nil {
		log.Printf("获取广告关键词配置失败: %v", err)
		return false
	}

	// 如果配置为空，返回false
	if adKeywordsStr == "" {
		return false
	}

	// 按逗号分割关键词（支持中文和英文逗号）
	adKeywords := q.splitKeywords(adKeywordsStr)

	return q.checkKeywordsInFilename(filename, adKeywords)
}

// checkKeywordsInFilename 检查文件名是否包含指定关键词
func (q *QuarkPanService) checkKeywordsInFilename(filename string, keywords []string) bool {
	// 转为小写进行比较
	lowercaseFilename := strings.ToLower(filename)

	for _, keyword := range keywords {
		if strings.Contains(lowercaseFilename, strings.ToLower(keyword)) {
			log.Printf("文件 %s 包含广告关键词: %s", filename, keyword)
			return true
		}
	}

	return false
}

// getSystemConfigValue 获取系统配置值
func (q *QuarkPanService) getSystemConfigValue(key string) (string, error) {
	// 检查是否需要刷新缓存
	select {
	case <-configRefreshChan:
		// 收到刷新信号，清空缓存
		systemConfigOnce.Do(func() {
			systemConfigRepo = repo.NewSystemConfigRepository(db.DB)
		})
		systemConfigRepo.ClearConfigCache()
	default:
		// 没有刷新信号，继续使用缓存
	}

	// 使用单例模式获取系统配置仓库
	systemConfigOnce.Do(func() {
		systemConfigRepo = repo.NewSystemConfigRepository(db.DB)
	})
	return systemConfigRepo.GetConfigValue(key)
}

// refreshSystemConfigCache 刷新系统配置缓存
func (q *QuarkPanService) refreshSystemConfigCache() {
	systemConfigOnce.Do(func() {
		systemConfigRepo = repo.NewSystemConfigRepository(db.DB)
	})
	systemConfigRepo.ClearConfigCache()
}

// RefreshSystemConfigCache 全局刷新系统配置缓存（供外部调用）
func RefreshSystemConfigCache() {
	select {
	case configRefreshChan <- true:
		// 发送刷新信号
	default:
		// 通道已满，忽略
	}
}

// splitKeywords 按逗号分割关键词（支持中文和英文逗号）
func (q *QuarkPanService) splitKeywords(keywordsStr string) []string {
	if keywordsStr == "" {
		return []string{}
	}

	// 使用正则表达式同时匹配中英文逗号
	re := regexp.MustCompile(`[,，]`)
	parts := re.Split(keywordsStr, -1)

	var result []string
	for _, part := range parts {
		// 去除首尾空格
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// splitAdURLs 按换行符分割广告URL列表
func (q *QuarkPanService) splitAdURLs(autoInsertAdStr string) []string {
	if autoInsertAdStr == "" {
		return []string{}
	}

	// 按换行符分割
	lines := strings.Split(autoInsertAdStr, "\n")
	var result []string

	for _, line := range lines {
		// 去除首尾空格
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// extractAdFileIDs 从广告URL列表中提取文件ID
func (q *QuarkPanService) extractAdFileIDs(adURLs []string) []string {
	var result []string

	for _, url := range adURLs {
		// 使用 ExtractShareIdString 提取分享ID
		shareID, _ := commonutils.ExtractShareIdString(url)
		if shareID != "" {
			result = append(result, shareID)
		}
	}

	return result
}

// addAd 添加个人自定义广告
func (q *QuarkPanService) addAd(dirID string) error {
	log.Printf("开始添加个人自定义广告到目录: %s", dirID)

	// 从系统配置中获取自动插入广告内容
	autoInsertAdStr, err := q.getSystemConfigValue(entity.ConfigKeyAutoInsertAd)
	if err != nil {
		log.Printf("获取自动插入广告配置失败: %v", err)
		return err
	}

	// 如果配置为空，跳过广告插入
	if autoInsertAdStr == "" {
		log.Printf("没有配置自动插入广告，跳过广告插入")
		return nil
	}

	// 按换行符分割广告URL列表
	adURLs := q.splitAdURLs(autoInsertAdStr)
	if len(adURLs) == 0 {
		log.Printf("没有有效的广告URL，跳过广告插入")
		return nil
	}

	// 提取广告文件ID列表
	adFileIDs := q.extractAdFileIDs(adURLs)
	if len(adFileIDs) == 0 {
		log.Printf("没有有效的广告文件ID，跳过广告插入")
		return nil
	}

	// 随机选择一个广告文件
	rand.Seed(utils.GetCurrentTimestampNano())
	selectedAdID := adFileIDs[rand.Intn(len(adFileIDs))]

	log.Printf("选择广告文件ID: %s", selectedAdID)

	// 获取广告文件的stoken
	stokenResult, err := q.getStoken(selectedAdID)
	if err != nil {
		log.Printf("获取广告文件stoken失败: %v", err)
		return err
	}

	// 获取广告文件详情
	adDetail, err := q.getShare(selectedAdID, stokenResult.Stoken)
	if err != nil {
		log.Printf("获取广告文件详情失败: %v", err)
		return err
	}

	if len(adDetail.List) == 0 {
		log.Printf("广告文件详情为空")
		return fmt.Errorf("广告文件详情为空")
	}

	// 获取第一个广告文件的信息
	adFile := adDetail.List[0]
	fid := adFile.Fid
	shareFidToken := adFile.ShareFidToken

	// 保存广告文件到目标目录
	saveResult, err := q.getShareSaveToDir(selectedAdID, stokenResult.Stoken, []string{fid}, []string{shareFidToken}, dirID)
	if err != nil {
		log.Printf("保存广告文件失败: %v", err)
		return err
	}

	// 等待保存完成
	_, err = q.waitForTask(saveResult.TaskID)
	if err != nil {
		log.Printf("等待广告文件保存完成失败: %v", err)
		return err
	}

	log.Printf("广告文件添加成功")
	return nil
}

// getDirFile 获取指定文件夹的文件列表
func (q *QuarkPanService) getDirFile(pdirFid string) ([]map[string]interface{}, error) {
	log.Printf("正在遍历父文件夹: %s", pdirFid)

	queryParams := map[string]string{
		"pr":              "ucpro",
		"fr":              "pc",
		"uc_param_str":    "",
		"pdir_fid":        pdirFid,
		"_page":           "1",
		"_size":           "50",
		"_fetch_total":    "1",
		"_fetch_sub_dirs": "0",
		"_sort":           "updated_at:desc",
	}

	respData, err := q.HTTPGet("https://drive-pc.quark.cn/1/clouddrive/file/sort", queryParams)
	if err != nil {
		log.Printf("获取目录文件失败: %v", err)
		return nil, err
	}

	var response struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			List []map[string]interface{} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		log.Printf("解析目录文件响应失败: %v", err)
		return nil, err
	}

	if response.Status != 200 {
		return nil, fmt.Errorf(response.Message)
	}

	// 直接返回文件列表，不递归处理子目录（与参考代码保持一致）
	return response.Data.List, nil
}

// 定义各种结果结构体
type StokenResult struct {
	Stoken string `json:"stoken"`
	Title  string `json:"title"`
}

type ShareResult struct {
	Share struct {
		Title string `json:"title"`
	} `json:"share"`
	List []struct {
		Fid           string `json:"fid"`
		ShareFidToken string `json:"share_fid_token"`
	} `json:"list"`
}

type SaveResult struct {
	TaskID string `json:"task_id"`
}

type ShareBtnResult struct {
	TaskID string `json:"task_id"`
}

type TaskResult struct {
	Status  int    `json:"status"`
	ShareID string `json:"share_id"`
	SaveAs  struct {
		SaveAsTopFids []string `json:"save_as_top_fids"`
	} `json:"save_as"`
}

type PasswordResult struct {
	ShareURL   string `json:"share_url"`
	ShareTitle string `json:"share_title"`
	Code       string `json:"code"`
	FirstFile  struct {
		Fid string `json:"fid"`
	} `json:"first_file"`
}

// GetUserInfo 获取用户信息
func (q *QuarkPanService) GetUserInfo(cookie *string) (*UserInfo, error) {
	// 临时设置cookie
	originalCookie := q.GetHeader("Cookie")
	q.SetHeader("Cookie", *cookie)
	defer q.SetHeader("Cookie", originalCookie) // 恢复原始cookie

	// 获取用户基本信息
	queryParams := map[string]string{
		"platform": "pc",
		"fr":       "pc",
	}

	data, err := q.HTTPGet("https://pan.quark.cn/account/info", queryParams)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}

	var response struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Data    struct {
			Nickname  string   `json:"nickname"`
			AvatarUri string   `json:"avatarUri"`
			Mobilekps string   `json:"mobilekps"`
			Config    struct{} `json:"config"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %v", err)
	}

	if !response.Success || response.Code != "OK" {
		return nil, fmt.Errorf("获取用户信息失败: API返回错误")
	}

	// 获取用户详细信息（容量和会员信息）
	queryParams1 := map[string]string{
		"pr":              "ucpro",
		"fr":              "pc",
		"uc_param_str":    "",
		"fetch_subscribe": "true",
		"_ch":             "home",
		"fetch_identity":  "true",
	}
	data1, err := q.HTTPGet("https://drive-pc.quark.cn/1/clouddrive/member", queryParams1)
	if err != nil {
		return nil, fmt.Errorf("获取用户详细信息失败: %v", err)
	}

	var memberResponse struct {
		Status  int    `json:"status"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TotalCapacity int64  `json:"total_capacity"`
			UseCapacity   int64  `json:"use_capacity"`
			MemberType    string `json:"member_type"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data1, &memberResponse); err != nil {
		return nil, fmt.Errorf("解析用户详细信息失败: %v", err)
	}

	if memberResponse.Status != 200 || memberResponse.Code != 0 {
		return nil, fmt.Errorf("获取用户详细信息失败: %s", memberResponse.Message)
	}

	// 判断VIP状态
	vipStatus := memberResponse.Data.MemberType != "NORMAL"

	return &UserInfo{
		Username:    response.Data.Nickname,
		VIPStatus:   vipStatus,
		UsedSpace:   memberResponse.Data.UseCapacity,
		TotalSpace:  memberResponse.Data.TotalCapacity,
		ServiceType: "quark",
	}, nil
}

func (xq *QuarkPanService) SetCKSRepository(cksRepo repo.CksRepository, entity entity.Cks) {
}

// UploadFile 上传本地文件到夸克网盘并生成分享链接
func (q *QuarkPanService) UploadFile(localFilePath string, pdirFid string) (*TransferResult, error) {
	if pdirFid == "" {
		pdirFid = "0"
	}

	// 1. 检查本地文件是否存在
	fileInfo, err := os.Stat(localFilePath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("本地文件不存在: %v", err)), nil
	}

	fileName := filepath.Base(localFilePath)
	fileSize := fileInfo.Size()

	log.Printf("开始上传文件: %s (大小: %s)", fileName, formatBytes(fileSize))

	// 2. 计算文件 SHA1 hash（用于秒传检测）
	sha1Hash, err := q.calculateFileSHA1(localFilePath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("计算文件SHA1失败: %v", err)), nil
	}
	log.Printf("文件SHA1: %s", sha1Hash)

	// 3. 预上传 - 检测秒传 / 获取上传URL
	uploadURL, isInstant, fid, err := q.preUpload(pdirFid, fileName, fileSize, sha1Hash)
	if err != nil {
		return ErrorResult(fmt.Sprintf("预上传失败: %v", err)), nil
	}

	if isInstant {
		// 秒传成功，文件已存在于网盘
		log.Printf("文件秒传成功，fid: %s", fid)
	} else {
		// 4. 上传文件内容
		if err := q.uploadFileContent(uploadURL, localFilePath, fileSize); err != nil {
			return ErrorResult(fmt.Sprintf("上传文件内容失败: %v", err)), nil
		}
		log.Printf("文件内容上传完成")

		// 5. 等待上传任务完成，获取 fid
		uploadTaskID := fid // preUpload 返回的 taskID
		if uploadTaskID == "" {
			return ErrorResult("上传任务ID为空"), nil
		}
		result, err := q.waitForTask(uploadTaskID)
		if err != nil {
			return ErrorResult(fmt.Sprintf("等待上传完成失败: %v", err)), nil
		}
		log.Printf("上传任务完成，结果: %+v", result)
	}

	// 6. 分享文件生成分享链接
	shareResult, err := q.shareUploadedFile(localFilePath)
	if err != nil {
		// 分享失败但上传成功，返回部分成功信息
		log.Printf("文件上传成功但分享失败: %v", err)
		return SuccessResult("上传成功（分享失败）", map[string]interface{}{
			"fileName":  fileName,
			"fileSize":  fileSize,
			"shareError": err.Error(),
		}), nil
	}

	return SuccessResult("上传并分享成功", map[string]interface{}{
		"fileName":  fileName,
		"fileSize":  fileSize,
		"shareUrl":  shareResult.ShareURL,
		"shareTitle": shareResult.ShareTitle,
		"code":      shareResult.Code,
		"fid":       shareResult.FirstFile.Fid,
	}), nil
}

// calculateFileSHA1 计算文件的SHA1哈希值
func (q *QuarkPanService) calculateFileSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// preUpload 夸克网盘预上传（秒传检测 + 获取上传URL）
// 返回: uploadURL, isInstant(是否秒传), taskID/fid, error
func (q *QuarkPanService) preUpload(pdirFid, fileName string, fileSize int64, sha1Hash string) (string, bool, string, error) {
	body := map[string]interface{}{
		"pdir_fid":  pdirFid,
		"file_name": fileName,
		"file_size": fileSize,
		"sha1":      sha1Hash,
	}

	queryParams := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
	}

	respData, err := q.HTTPPost("https://drive-pc.quark.cn/1/clouddrive/file", body, queryParams)
	if err != nil {
		return "", false, "", fmt.Errorf("预上传请求失败: %v", err)
	}

	var response struct {
		Status  int    `json:"status"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID    string `json:"task_id"`
			UploadURL string `json:"upload_url"`
			Fid       string `json:"fid"`
			Finish    bool   `json:"finish"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return "", false, "", fmt.Errorf("解析预上传响应失败: %v", err)
	}

	if response.Status != 200 && response.Code != 0 {
		msg := response.Message
		if msg == "" {
			msg = response.Data.TaskID
		}
		return "", false, "", fmt.Errorf("预上传失败: %s", msg)
	}

	// 秒传：文件已存在
	if response.Data.Finish && response.Data.Fid != "" {
		return "", true, response.Data.Fid, nil
	}

	return response.Data.UploadURL, false, response.Data.TaskID, nil
}

// uploadFileContent 上传文件内容到夸克网盘
func (q *QuarkPanService) uploadFileContent(uploadURL, localFilePath string, fileSize int64) error {
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %v", err)
	}
	defer file.Close()

	req, err := http.NewRequest("PUT", uploadURL, file)
	if err != nil {
		return fmt.Errorf("创建上传请求失败: %v", err)
	}

	req.ContentLength = fileSize
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("上传文件请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("上传文件失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	return nil
}

// shareUploadedFile 为刚上传的文件生成分享链接
// 先获取最近上传的文件，然后对其分享
func (q *QuarkPanService) shareUploadedFile(localFileName string) (*PasswordResult, error) {
	// 获取根目录最近的文件列表
	fileResult, err := q.GetFiles("0")
	if err != nil {
		return nil, fmt.Errorf("获取文件列表失败: %v", err)
	}

	if fileResult == nil || !fileResult.Success {
		return nil, fmt.Errorf("获取文件列表返回失败")
	}

	fileList, ok := fileResult.Data.([]interface{})
	if !ok || len(fileList) == 0 {
		return nil, fmt.Errorf("文件列表为空")
	}

	// 找到刚上传的文件（匹配文件名）
	var targetFid string
	var targetName string
	for _, item := range fileList {
		if fileMap, ok := item.(map[string]interface{}); ok {
			if name, ok := fileMap["file_name"].(string); ok && name == filepath.Base(localFileName) {
				targetFid, _ = fileMap["fid"].(string)
				targetName = name
				break
			}
		}
	}

	if targetFid == "" {
		return nil, fmt.Errorf("未在文件列表中找到刚上传的文件: %s", localFileName)
	}

	log.Printf("找到文件: %s, fid: %s, 开始生成分享链接", targetName, targetFid)

	// 使用现有的分享方法
	shareBtnResult, err := q.getShareBtn([]string{targetFid}, targetName)
	if err != nil {
		return nil, fmt.Errorf("创建分享失败: %v", err)
	}

	shareTaskResult, err := q.waitForTask(shareBtnResult.TaskID)
	if err != nil {
		return nil, fmt.Errorf("等待分享完成失败: %v", err)
	}

	passwordResult, err := q.getSharePassword(shareTaskResult.ShareID)
	if err != nil {
		return nil, fmt.Errorf("获取分享密码失败: %v", err)
	}

	return passwordResult, nil
}

// formatBytes 格式化字节数为可读格式
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Mkdir 创建文件夹
// parentFid: 父目录ID, folderName: 文件夹名称
// 返回: 新文件夹的 fid
func (q *QuarkPanService) Mkdir(parentFid, folderName string) (string, error) {
	if parentFid == "" {
		parentFid = "0"
	}

	body := map[string]interface{}{
		"pdir_fid":  parentFid,
		"file_name": folderName,
		"dir":       1,
	}

	queryParams := map[string]string{
		"pr":           "ucpro",
		"fr":           "pc",
		"uc_param_str": "",
	}

	respData, err := q.HTTPPost("https://drive-pc.quark.cn/1/clouddrive/file", body, queryParams)
	if err != nil {
		return "", fmt.Errorf("创建文件夹请求失败: %v", err)
	}

	var response struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Fid    string `json:"fid"`
			Finish bool   `json:"finish"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return "", fmt.Errorf("解析创建文件夹响应失败: %v", err)
	}

	if response.Status != 200 {
		return "", fmt.Errorf("创建文件夹失败: %s", response.Message)
	}

	log.Printf("文件夹创建成功: %s, fid: %s", folderName, response.Data.Fid)
	return response.Data.Fid, nil
}

// ShareFolder 分享文件夹（获取文件夹下所有文件并创建分享）
// 返回: 分享链接和密码
func (q *QuarkPanService) ShareFolder(folderFid, title string) (*PasswordResult, error) {
	// 1. 获取文件夹下所有文件
	fileResult, err := q.GetFiles(folderFid)
	if err != nil {
		return nil, fmt.Errorf("获取文件夹内容失败: %v", err)
	}

	if fileResult == nil || !fileResult.Success {
		return nil, fmt.Errorf("获取文件夹内容失败")
	}

	fileList, ok := fileResult.Data.([]interface{})
	if !ok || len(fileList) == 0 {
		return nil, fmt.Errorf("文件夹为空，无法分享")
	}

	// 收集所有文件 fid
	fidList := make([]string, 0, len(fileList))
	for _, item := range fileList {
		if fileMap, ok := item.(map[string]interface{}); ok {
			if fid, ok := fileMap["fid"].(string); ok && fid != "" {
				fidList = append(fidList, fid)
			}
		}
	}

	if len(fidList) == 0 {
		return nil, fmt.Errorf("文件夹中没有有效文件")
	}

	log.Printf("准备分享文件夹: %s, 包含 %d 个文件, fids: %v", title, len(fidList), fidList)

	// 2. 创建分享
	shareBtnResult, err := q.getShareBtn(fidList, title)
	if err != nil {
		return nil, fmt.Errorf("创建分享失败: %v", err)
	}

	// 3. 等待分享任务完成
	shareTaskResult, err := q.waitForTask(shareBtnResult.TaskID)
	if err != nil {
		return nil, fmt.Errorf("等待分享完成失败: %v", err)
	}

	// 4. 获取分享链接和密码
	passwordResult, err := q.getSharePassword(shareTaskResult.ShareID)
	if err != nil {
		return nil, fmt.Errorf("获取分享密码失败: %v", err)
	}

	log.Printf("文件夹分享成功: %s → %s (密码: %s)", title, passwordResult.ShareURL, passwordResult.Code)
	return passwordResult, nil
}
