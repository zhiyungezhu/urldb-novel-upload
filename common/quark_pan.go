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

// QuarkPanService ������̷���
type QuarkPanService struct {
	*BasePanService
	configMutex sync.RWMutex // �������õĶ�д��
}

// ȫ�����û���ˢ���ź�
var configRefreshChan = make(chan bool, 1)

// ������ر���
var (
	systemConfigRepo repo.SystemConfigRepository
	systemConfigOnce sync.Once
)

// NewQuarkPanService ����������̷��񣨵���ģʽ��
func NewQuarkPanService(config *PanConfig) *QuarkPanService {
	quarkInstance := &QuarkPanService{
		BasePanService: NewBasePanService(config),
	}

	// ���ÿ�����̵�Ĭ������ͷ
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

	// ��������
	quarkInstance.UpdateConfig(config)

	return quarkInstance
}

// GetQuarkInstance ��ȡ������̷�����ʵ��
func GetQuarkInstance() *QuarkPanService {
	return NewQuarkPanService(nil)
}

// UpdateConfig �������ã��̰߳�ȫ��
func (q *QuarkPanService) UpdateConfig(config *PanConfig) {
	if config == nil {
		return
	}

	q.configMutex.Lock()
	defer q.configMutex.Unlock()

	q.config = config
	// ����Cookie��header
	if config.Cookie != "" {
		q.SetHeader("Cookie", config.Cookie)
	}
}

// SetCookie ����Cookie
func (q *QuarkPanService) SetCookie(cookie string) {
	q.SetHeader("Cookie", cookie)
	q.configMutex.Lock()
	if q.config != nil {
		q.config.Cookie = cookie
	}
	q.configMutex.Unlock()
}

// GetCookie ��ȡ��ǰCookie
func (q *QuarkPanService) GetCookie() string {
	return q.GetHeader("Cookie")
}

// GetServiceType ��ȡ��������
func (q *QuarkPanService) GetServiceType() ServiceType {
	return Quark
}

// Transfer ת���������
func (q *QuarkPanService) Transfer(shareID string) (*TransferResult, error) {
	// ��ȡ���ã��̰߳�ȫ��
	q.configMutex.RLock()
	config := q.config
	q.configMutex.RUnlock()

	log.Printf("��ʼ������˷���: %s", shareID)

	// ��ȡstoken
	var stoken string
	if config.Stoken == "" {
		stokenResult, err := q.getStoken(shareID)
		if err != nil {
			return ErrorResult(fmt.Sprintf("��ȡstokenʧ��: %v", err)), nil
		}

		stoken = strings.ReplaceAll(stokenResult.Stoken, " ", "+")
	} else {
		stoken = strings.ReplaceAll(config.Stoken, " ", "+")
	}

	// ��ȡ��������
	shareResult, err := q.getShare(shareID, stoken)
	if err != nil || len(shareResult.List) == 0 {
		return ErrorResult(fmt.Sprintf("��ȡ��������ʧ��: %v", err)), nil
	}

	if config.IsType == 1 {
		// ���� ��ԴĿ¼�ṹ
		for _, item := range shareResult.List {
			// ��ȡ�ļ���Ϣ
			fileList, err := q.getDirFile(item.Fid)
			if err != nil {
				log.Printf("��ȡĿ¼�ļ�ʧ��: %v", err)
				continue
			}

			// �����ļ��б�
			if fileList != nil {
				log.Printf("Ŀ¼ %s ���� %d ���ļ�/�ļ���", item.Fid, len(fileList))

				// ���������ļ����������������Ӿ���Ĵ����߼�
				for _, file := range fileList {
					if fileName, ok := file["file_name"].(string); ok {
						if fileType, ok := file["file_type"].(float64); ok {
							fileTypeStr := "�ļ�"
							if fileType == 1 {
								fileTypeStr = "Ŀ¼"
							}
							log.Printf("  - %s (%s)", fileName, fileTypeStr)
						}
					}
				}
			}
		}

		// ֱ�ӷ�����Դ��Ϣ
		return SuccessResult("����ɹ�", map[string]interface{}{
			"title":    shareResult.Share.Title,
			"shareUrl": config.URL,
		}), nil
	}

	// ��ȡ�ļ���Ϣ
	fidList := make([]string, 0)
	fidTokenList := make([]string, 0)
	title := shareResult.Share.Title

	for _, item := range shareResult.List {
		fidList = append(fidList, item.Fid)
		fidTokenList = append(fidTokenList, item.ShareFidToken)
	}

	// ת����Դ
	saveResult, err := q.getShareSave(shareID, stoken, fidList, fidTokenList)
	if err != nil {
		return ErrorResult(fmt.Sprintf("ת��ʧ��: %v", err)), nil
	}

	taskID := saveResult.TaskID

	// �ȴ�ת�����
	myData, err := q.waitForTask(taskID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("�ȴ�ת�����ʧ��: %v", err)), nil
	}

	// ɾ������ļ�����������ã�
	if err := q.deleteAdFiles(myData.SaveAs.SaveAsTopFids[0]); err != nil {
		log.Printf("ɾ������ļ�ʧ��: %v", err)
	}

	// ���Ӹ����Զ�����
	if err := q.addAd(myData.SaveAs.SaveAsTopFids[0]); err != nil {
		log.Printf("���ӹ���ļ�ʧ��: %v", err)
	}

	// ������Դ
	shareBtnResult, err := q.getShareBtn(myData.SaveAs.SaveAsTopFids, title)
	if err != nil {
		return ErrorResult(fmt.Sprintf("����ʧ��: %v", err)), nil
	}

	// �ȴ��������
	shareTaskResult, err := q.waitForTask(shareBtnResult.TaskID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("�ȴ��������ʧ��: %v", err)), nil
	}

	// ��ȡ��������
	passwordResult, err := q.getSharePassword(shareTaskResult.ShareID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡ��������ʧ��: %v", err)), nil
	}

	// ȷ��fid
	var fid string
	if len(myData.SaveAs.SaveAsTopFids) > 1 {
		fid = strings.Join(myData.SaveAs.SaveAsTopFids, ",")
	} else {
		fid = passwordResult.FirstFile.Fid
	}

	return SuccessResult("ת��ɹ�", map[string]interface{}{
		"shareUrl": passwordResult.ShareURL,
		"title":    passwordResult.ShareTitle,
		"fid":      fid,
		"code":     passwordResult.Code,
	}), nil
}

// GetFiles ��ȡ�ļ��б�
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
		return ErrorResult(fmt.Sprintf("��ȡ�ļ��б�ʧ��: %v", err)), nil
	}

	var response struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			List []interface{} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return ErrorResult("������Ӧʧ��"), nil
	}

	if response.Status != 200 {
		message := response.Message
		if message == "require login [guest]" {
			message = "���δ��¼������cookie"
		}
		return ErrorResult(message), nil
	}

	return SuccessResult("��ȡ�ɹ�", response.Data.List), nil
}

// DeleteFiles ɾ���ļ�
func (q *QuarkPanService) DeleteFiles(fileList []string) (*TransferResult, error) {
	if len(fileList) == 0 {
		return ErrorResult("�ļ��б�Ϊ��"), nil
	}

	// ���ɾ���ļ���ȷ��ÿ��ɾ�����������
	for _, fileID := range fileList {
		err := q.deleteSingleFile(fileID)
		if err != nil {
			log.Printf("ɾ���ļ� %s ʧ��: %v", fileID, err)
			return ErrorResult(fmt.Sprintf("ɾ���ļ� %s ʧ��: %v", fileID, err)), nil
		}
	}

	return SuccessResult("ɾ���ɹ�", nil), nil
}

// deleteSingleFile ɾ�������ļ�
func (q *QuarkPanService) deleteSingleFile(fileID string) error {
	log.Printf("����ɾ���ļ�: %s", fileID)

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
		return fmt.Errorf("ɾ���ļ�����ʧ��: %v", err)
	}

	// ������Ӧ
	var response struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return fmt.Errorf("����ɾ����Ӧʧ��: %v", err)
	}

	if response.Status != 200 {
		return fmt.Errorf("ɾ���ļ�ʧ��: %s", response.Message)
	}

	// ���������ID���ȴ��������
	if response.Data.TaskID != "" {
		log.Printf("ɾ���ļ�����ID: %s", response.Data.TaskID)
		_, err := q.waitForTask(response.Data.TaskID)
		if err != nil {
			return fmt.Errorf("�ȴ�ɾ���������ʧ��: %v", err)
		}
		log.Printf("�ļ� %s ɾ�����", fileID)
	} else {
		log.Printf("�ļ� %s ɾ����ɣ�������ID��", fileID)
	}

	return nil
}

// getStoken ��ȡstoken
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

// getShare ��ȡ��������
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

// getShareSave ת�����
func (q *QuarkPanService) getShareSave(shareID, stoken string, fidList, fidTokenList []string) (*SaveResult, error) {
	return q.getShareSaveToDir(shareID, stoken, fidList, fidTokenList, "0")
}

// getShareSaveToDir ת�������ָ��Ŀ¼
func (q *QuarkPanService) getShareSaveToDir(shareID, stoken string, fidList, fidTokenList []string, toPdirFid string) (*SaveResult, error) {
	data := map[string]interface{}{
		"pwd_id":         shareID,
		"stoken":         stoken,
		"fid_list":       fidList,
		"fid_token_list": fidTokenList,
		"to_pdir_fid":    toPdirFid, // �洢��ָ��Ŀ¼
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

// ����ָ�����ȵ�ʱ���
func (q *QuarkPanService) generateTimestamp(length int) int64 {
	timestamp := utils.GetCurrentTime().UnixNano() / int64(time.Millisecond)
	timestampStr := strconv.FormatInt(timestamp, 10)
	if len(timestampStr) > length {
		timestampStr = timestampStr[:length]
	}
	timestamp, _ = strconv.ParseInt(timestampStr, 10, 64)
	return timestamp
}

// getShareBtn ������ť
func (q *QuarkPanService) getShareBtn(fidList []string, title string) (*ShareBtnResult, error) {
	data := map[string]interface{}{
		"fid_list":     fidList,
		"title":        title,
		"url_type":     1,
		"expired_type": 1, // ���÷���
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

// getShareTask ��ȡ��������״̬
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

// getSharePassword ��ȡ��������
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

// waitForTask �ȴ��������
func (q *QuarkPanService) waitForTask(taskID string) (*TaskResult, error) {
	maxRetries := 50
	retryDelay := 2 * time.Second

	for retryIndex := 0; retryIndex < maxRetries; retryIndex++ {
		result, err := q.getShareTask(taskID, retryIndex)
		if err != nil {
			if strings.Contains(err.Error(), "capacity limit[{0}]") {
				return nil, fmt.Errorf("��������")
			}
			return nil, err
		}

		if result.Status == 2 { // �������
			return result, nil
		}

		time.Sleep(retryDelay)
	}

	return nil, fmt.Errorf("����ʱ")
}

// deleteAdFiles ɾ������ļ�
func (q *QuarkPanService) deleteAdFiles(pdirFid string) error {
	log.Printf("��ʼɾ������ļ���Ŀ¼ID: %s", pdirFid)

	// ��ȡĿ¼�ļ��б�
	fileList, err := q.getDirFile(pdirFid)
	if err != nil {
		log.Printf("��ȡĿ¼�ļ�ʧ��: %v", err)
		return err
	}

	if fileList == nil || len(fileList) == 0 {
		log.Printf("Ŀ¼Ϊ�գ�����ɾ������ļ�")
		return nil
	}

	// ɾ���������ؼ��ʵ��ļ�
	for _, file := range fileList {
		if fileName, ok := file["file_name"].(string); ok {
			log.Printf("����ļ�: %s", fileName)
			if q.containsAdKeywords(fileName) {
				if fid, ok := file["fid"].(string); ok {
					log.Printf("ɾ������ļ�: %s (FID: %s)", fileName, fid)
					_, err := q.DeleteFiles([]string{fid})
					if err != nil {
						log.Printf("ɾ������ļ�ʧ��: %v", err)
					} else {
						log.Printf("�ɹ�ɾ������ļ�: %s", fileName)
					}
				}
			}
		}
	}

	return nil
}

// containsAdKeywords ����ļ����Ƿ�������ؼ���
func (q *QuarkPanService) containsAdKeywords(filename string) bool {
	// ��ϵͳ�����л�ȡ���ؼ���
	adKeywordsStr, err := q.getSystemConfigValue(entity.ConfigKeyAdKeywords)
	if err != nil {
		log.Printf("��ȡ���ؼ�������ʧ��: %v", err)
		return false
	}

	// �������Ϊ�գ�����false
	if adKeywordsStr == "" {
		return false
	}

	// �����ŷָ�ؼ��ʣ�֧�����ĺ�Ӣ�Ķ��ţ�
	adKeywords := q.splitKeywords(adKeywordsStr)

	return q.checkKeywordsInFilename(filename, adKeywords)
}

// checkKeywordsInFilename ����ļ����Ƿ����ָ���ؼ���
func (q *QuarkPanService) checkKeywordsInFilename(filename string, keywords []string) bool {
	// תΪСд���бȽ�
	lowercaseFilename := strings.ToLower(filename)

	for _, keyword := range keywords {
		if strings.Contains(lowercaseFilename, strings.ToLower(keyword)) {
			log.Printf("�ļ� %s �������ؼ���: %s", filename, keyword)
			return true
		}
	}

	return false
}

// getSystemConfigValue ��ȡϵͳ����ֵ
func (q *QuarkPanService) getSystemConfigValue(key string) (string, error) {
	// ����Ƿ���Ҫˢ�»���
	select {
	case <-configRefreshChan:
		// �յ�ˢ���źţ���ջ���
		systemConfigOnce.Do(func() {
			systemConfigRepo = repo.NewSystemConfigRepository(db.DB)
		})
		systemConfigRepo.ClearConfigCache()
	default:
		// û��ˢ���źţ�����ʹ�û���
	}

	// ʹ�õ���ģʽ��ȡϵͳ���òֿ�
	systemConfigOnce.Do(func() {
		systemConfigRepo = repo.NewSystemConfigRepository(db.DB)
	})
	return systemConfigRepo.GetConfigValue(key)
}

// refreshSystemConfigCache ˢ��ϵͳ���û���
func (q *QuarkPanService) refreshSystemConfigCache() {
	systemConfigOnce.Do(func() {
		systemConfigRepo = repo.NewSystemConfigRepository(db.DB)
	})
	systemConfigRepo.ClearConfigCache()
}

// RefreshSystemConfigCache ȫ��ˢ��ϵͳ���û��棨���ⲿ���ã�
func RefreshSystemConfigCache() {
	select {
	case configRefreshChan <- true:
		// ����ˢ���ź�
	default:
		// ͨ������������
	}
}

// splitKeywords �����ŷָ�ؼ��ʣ�֧�����ĺ�Ӣ�Ķ��ţ�
func (q *QuarkPanService) splitKeywords(keywordsStr string) []string {
	if keywordsStr == "" {
		return []string{}
	}

	// ʹ���������ʽͬʱƥ����Ӣ�Ķ���
	re := regexp.MustCompile(`[,��]`)
	parts := re.Split(keywordsStr, -1)

	var result []string
	for _, part := range parts {
		// ȥ����β�ո�
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// splitAdURLs �����з��ָ���URL�б�
func (q *QuarkPanService) splitAdURLs(autoInsertAdStr string) []string {
	if autoInsertAdStr == "" {
		return []string{}
	}

	// �����з��ָ�
	lines := strings.Split(autoInsertAdStr, "\n")
	var result []string

	for _, line := range lines {
		// ȥ����β�ո�
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// extractAdFileIDs �ӹ��URL�б�����ȡ�ļ�ID
func (q *QuarkPanService) extractAdFileIDs(adURLs []string) []string {
	var result []string

	for _, url := range adURLs {
		// ʹ�� ExtractShareIdString ��ȡ����ID
		shareID, _ := commonutils.ExtractShareIdString(url)
		if shareID != "" {
			result = append(result, shareID)
		}
	}

	return result
}

// addAd ���Ӹ����Զ�����
func (q *QuarkPanService) addAd(dirID string) error {
	log.Printf("��ʼ���Ӹ����Զ����浽Ŀ¼: %s", dirID)

	// ��ϵͳ�����л�ȡ�Զ�����������
	autoInsertAdStr, err := q.getSystemConfigValue(entity.ConfigKeyAutoInsertAd)
	if err != nil {
		log.Printf("��ȡ�Զ�����������ʧ��: %v", err)
		return err
	}

	// �������Ϊ�գ�����������
	if autoInsertAdStr == "" {
		log.Printf("û�������Զ������棬����������")
		return nil
	}

	// �����з��ָ���URL�б�
	adURLs := q.splitAdURLs(autoInsertAdStr)
	if len(adURLs) == 0 {
		log.Printf("û����Ч�Ĺ��URL������������")
		return nil
	}

	// ��ȡ����ļ�ID�б�
	adFileIDs := q.extractAdFileIDs(adURLs)
	if len(adFileIDs) == 0 {
		log.Printf("û����Ч�Ĺ���ļ�ID������������")
		return nil
	}

	// ���ѡ��һ������ļ�
	rand.Seed(utils.GetCurrentTimestampNano())
	selectedAdID := adFileIDs[rand.Intn(len(adFileIDs))]

	log.Printf("ѡ�����ļ�ID: %s", selectedAdID)

	// ��ȡ����ļ���stoken
	stokenResult, err := q.getStoken(selectedAdID)
	if err != nil {
		log.Printf("��ȡ����ļ�stokenʧ��: %v", err)
		return err
	}

	// ��ȡ����ļ�����
	adDetail, err := q.getShare(selectedAdID, stokenResult.Stoken)
	if err != nil {
		log.Printf("��ȡ����ļ�����ʧ��: %v", err)
		return err
	}

	if len(adDetail.List) == 0 {
		log.Printf("����ļ�����Ϊ��")
		return fmt.Errorf("����ļ�����Ϊ��")
	}

	// ��ȡ��һ������ļ�����Ϣ
	adFile := adDetail.List[0]
	fid := adFile.Fid
	shareFidToken := adFile.ShareFidToken

	// �������ļ���Ŀ��Ŀ¼
	saveResult, err := q.getShareSaveToDir(selectedAdID, stokenResult.Stoken, []string{fid}, []string{shareFidToken}, dirID)
	if err != nil {
		log.Printf("�������ļ�ʧ��: %v", err)
		return err
	}

	// �ȴ��������
	_, err = q.waitForTask(saveResult.TaskID)
	if err != nil {
		log.Printf("�ȴ�����ļ��������ʧ��: %v", err)
		return err
	}

	log.Printf("����ļ����ӳɹ�")
	return nil
}

// getDirFile ��ȡָ���ļ��е��ļ��б�
func (q *QuarkPanService) getDirFile(pdirFid string) ([]map[string]interface{}, error) {
	log.Printf("���ڱ������ļ���: %s", pdirFid)

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
		log.Printf("��ȡĿ¼�ļ�ʧ��: %v", err)
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
		log.Printf("����Ŀ¼�ļ���Ӧʧ��: %v", err)
		return nil, err
	}

	if response.Status != 200 {
		return nil, fmt.Errorf(response.Message)
	}

	// ֱ�ӷ����ļ��б������ݹ鴦����Ŀ¼����ο����뱣��һ�£�
	return response.Data.List, nil
}

// ������ֽ���ṹ��
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

// GetUserInfo ��ȡ�û���Ϣ
func (q *QuarkPanService) GetUserInfo(cookie *string) (*UserInfo, error) {
	// ��ʱ����cookie
	originalCookie := q.GetHeader("Cookie")
	q.SetHeader("Cookie", *cookie)
	defer q.SetHeader("Cookie", originalCookie) // �ָ�ԭʼcookie

	// ��ȡ�û�������Ϣ
	queryParams := map[string]string{
		"platform": "pc",
		"fr":       "pc",
	}

	data, err := q.HTTPGet("https://pan.quark.cn/account/info", queryParams)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�û���Ϣʧ��: %v", err)
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
		return nil, fmt.Errorf("�����û���Ϣʧ��: %v", err)
	}

	if !response.Success || response.Code != "OK" {
		return nil, fmt.Errorf("��ȡ�û���Ϣʧ��: API���ش���")
	}

	// ��ȡ�û���ϸ��Ϣ�������ͻ�Ա��Ϣ��
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
		return nil, fmt.Errorf("��ȡ�û���ϸ��Ϣʧ��: %v", err)
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
		return nil, fmt.Errorf("�����û���ϸ��Ϣʧ��: %v", err)
	}

	if memberResponse.Status != 200 || memberResponse.Code != 0 {
		return nil, fmt.Errorf("��ȡ�û���ϸ��Ϣʧ��: %s", memberResponse.Message)
	}

	// �ж�VIP״̬
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

// UploadFile �ϴ������ļ���������̲����ɷ�������
func (q *QuarkPanService) UploadFile(localFilePath string, pdirFid string) (*TransferResult, error) {
	if pdirFid == "" {
		pdirFid = "0"
	}

	// 1. ��鱾���ļ��Ƿ����
	fileInfo, err := os.Stat(localFilePath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("�����ļ�������: %v", err)), nil
	}

	fileName := filepath.Base(localFilePath)
	fileSize := fileInfo.Size()

	log.Printf("��ʼ�ϴ��ļ�: %s (��С: %s)", fileName, formatBytes(fileSize))

	// 2. �����ļ� SHA1 hash�������봫��⣩
	sha1Hash, err := q.calculateFileSHA1(localFilePath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("�����ļ�SHA1ʧ��: %v", err)), nil
	}
	log.Printf("�ļ�SHA1: %s", sha1Hash)

	// 3. Ԥ�ϴ� - ����봫 / ��ȡ�ϴ�URL
	uploadURL, isInstant, fid, err := q.preUpload(pdirFid, fileName, fileSize, sha1Hash)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Ԥ�ϴ�ʧ��: %v", err)), nil
	}

	if isInstant {
		// �봫�ɹ����ļ��Ѵ���������
		log.Printf("�ļ��봫�ɹ���fid: %s", fid)
	} else {
		// 4. �ϴ��ļ�����
		if err := q.uploadFileContent(uploadURL, localFilePath, fileSize); err != nil {
			return ErrorResult(fmt.Sprintf("�ϴ��ļ�����ʧ��: %v", err)), nil
		}
		log.Printf("�ļ������ϴ����")

		// 5. �ȴ��ϴ�������ɣ���ȡ fid
		uploadTaskID := fid // preUpload ���ص� taskID
		if uploadTaskID == "" {
			return ErrorResult("�ϴ�����IDΪ��"), nil
		}
		result, err := q.waitForTask(uploadTaskID)
		if err != nil {
			return ErrorResult(fmt.Sprintf("�ȴ��ϴ����ʧ��: %v", err)), nil
		}
		log.Printf("�ϴ�������ɣ����: %+v", result)
	}

	// 6. �����ļ����ɷ�������
	shareResult, err := q.shareUploadedFile(localFilePath)
	if err != nil {
		// ����ʧ�ܵ��ϴ��ɹ������ز��ֳɹ���Ϣ
		log.Printf("�ļ��ϴ��ɹ�������ʧ��: %v", err)
		return SuccessResult("�ϴ��ɹ�������ʧ�ܣ�", map[string]interface{}{
			"fileName":  fileName,
			"fileSize":  fileSize,
			"shareError": err.Error(),
		}), nil
	}

	return SuccessResult("�ϴ��������ɹ�", map[string]interface{}{
		"fileName":  fileName,
		"fileSize":  fileSize,
		"shareUrl":  shareResult.ShareURL,
		"shareTitle": shareResult.ShareTitle,
		"code":      shareResult.Code,
		"fid":       shareResult.FirstFile.Fid,
	}), nil
}

// calculateFileSHA1 �����ļ���SHA1��ϣֵ
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

// preUpload �������Ԥ�ϴ����봫��� + ��ȡ�ϴ�URL��
// ����: uploadURL, isInstant(�Ƿ��봫), taskID/fid, error
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
		return "", false, "", fmt.Errorf("Ԥ�ϴ�����ʧ��: %v", err)
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
		return "", false, "", fmt.Errorf("����Ԥ�ϴ���Ӧʧ��: %v", err)
	}

	if response.Status != 200 && response.Code != 0 {
		msg := response.Message
		if msg == "" {
			msg = response.Data.TaskID
		}
		return "", false, "", fmt.Errorf("Ԥ�ϴ�ʧ��: %s", msg)
	}

	// �봫���ļ��Ѵ���
	if response.Data.Finish && response.Data.Fid != "" {
		return "", true, response.Data.Fid, nil
	}

	return response.Data.UploadURL, false, response.Data.TaskID, nil
}

// uploadFileContent �ϴ��ļ����ݵ��������
func (q *QuarkPanService) uploadFileContent(uploadURL, localFilePath string, fileSize int64) error {
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("�򿪱����ļ�ʧ��: %v", err)
	}
	defer file.Close()

	req, err := http.NewRequest("PUT", uploadURL, file)
	if err != nil {
		return fmt.Errorf("�����ϴ�����ʧ��: %v", err)
	}

	req.ContentLength = fileSize
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("�ϴ��ļ�����ʧ��: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("�ϴ��ļ�ʧ�ܣ�״̬��: %d, ��Ӧ: %s", resp.StatusCode, string(body))
	}

	return nil
}

// shareUploadedFile Ϊ���ϴ����ļ����ɷ�������
// �Ȼ�ȡ����ϴ����ļ���Ȼ��������
func (q *QuarkPanService) shareUploadedFile(localFileName string) (*PasswordResult, error) {
	// ��ȡ��Ŀ¼������ļ��б�
	fileResult, err := q.GetFiles("0")
	if err != nil {
		return nil, fmt.Errorf("��ȡ�ļ��б�ʧ��: %v", err)
	}

	if fileResult == nil || !fileResult.Success {
		return nil, fmt.Errorf("��ȡ�ļ��б�����ʧ��")
	}

	fileList, ok := fileResult.Data.([]interface{})
	if !ok || len(fileList) == 0 {
		return nil, fmt.Errorf("�ļ��б�Ϊ��")
	}

	// �ҵ����ϴ����ļ���ƥ���ļ�����
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
		return nil, fmt.Errorf("δ���ļ��б����ҵ����ϴ����ļ�: %s", localFileName)
	}

	log.Printf("�ҵ��ļ�: %s, fid: %s, ��ʼ���ɷ�������", targetName, targetFid)

	// ʹ�����еķ�������
	shareBtnResult, err := q.getShareBtn([]string{targetFid}, targetName)
	if err != nil {
		return nil, fmt.Errorf("��������ʧ��: %v", err)
	}

	shareTaskResult, err := q.waitForTask(shareBtnResult.TaskID)
	if err != nil {
		return nil, fmt.Errorf("�ȴ��������ʧ��: %v", err)
	}

	passwordResult, err := q.getSharePassword(shareTaskResult.ShareID)
	if err != nil {
		return nil, fmt.Errorf("��ȡ��������ʧ��: %v", err)
	}

	return passwordResult, nil
}

// formatBytes ��ʽ���ֽ���Ϊ�ɶ���ʽ
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

// Mkdir �����ļ���
// parentFid: ��Ŀ¼ID, folderName: �ļ�������
// ����: ���ļ��е� fid
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
		return "", fmt.Errorf("�����ļ�������ʧ��: %v", err)
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
		return "", fmt.Errorf("���������ļ�����Ӧʧ��: %v", err)
	}

	if response.Status != 200 {
		return "", fmt.Errorf("�����ļ���ʧ��: %s", response.Message)
	}

	log.Printf("�ļ��д����ɹ�: %s, fid: %s", folderName, response.Data.Fid)
	return response.Data.Fid, nil
}

// ShareFolder �����ļ��У���ȡ�ļ����������ļ�������������
// ����: �������Ӻ�����
func (q *QuarkPanService) ShareFolder(folderFid, title string) (*PasswordResult, error) {
	// 1. ��ȡ�ļ����������ļ�
	fileResult, err := q.GetFiles(folderFid)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�ļ�������ʧ��: %v", err)
	}

	if fileResult == nil || !fileResult.Success {
		return nil, fmt.Errorf("��ȡ�ļ�������ʧ��")
	}

	fileList, ok := fileResult.Data.([]interface{})
	if !ok || len(fileList) == 0 {
		return nil, fmt.Errorf("�ļ���Ϊ�գ��޷�����")
	}

	// �ռ������ļ� fid
	fidList := make([]string, 0, len(fileList))
	for _, item := range fileList {
		if fileMap, ok := item.(map[string]interface{}); ok {
			if fid, ok := fileMap["fid"].(string); ok && fid != "" {
				fidList = append(fidList, fid)
			}
		}
	}

	if len(fidList) == 0 {
		return nil, fmt.Errorf("�ļ�����û����Ч�ļ�")
	}

	log.Printf("׼�������ļ���: %s, ���� %d ���ļ�, fids: %v", title, len(fidList), fidList)

	// 2. ��������
	shareBtnResult, err := q.getShareBtn(fidList, title)
	if err != nil {
		return nil, fmt.Errorf("��������ʧ��: %v", err)
	}

	// 3. �ȴ������������
	shareTaskResult, err := q.waitForTask(shareBtnResult.TaskID)
	if err != nil {
		return nil, fmt.Errorf("�ȴ��������ʧ��: %v", err)
	}

	// 4. ��ȡ�������Ӻ�����
	passwordResult, err := q.getSharePassword(shareTaskResult.ShareID)
	if err != nil {
		return nil, fmt.Errorf("��ȡ��������ʧ��: %v", err)
	}

	log.Printf("�ļ��з����ɹ�: %s �� %s (����: %s)", title, passwordResult.ShareURL, passwordResult.Code)
	return passwordResult, nil
}
