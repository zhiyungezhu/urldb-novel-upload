package pan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
	"github.com/zhiyungezhu/urldb-novel-upload/utils"
)

// AlipanService �������̷���
type AlipanService struct {
	*BasePanService
	accessToken string
	configMutex sync.RWMutex // �������õĶ�д��
}

// ������ر���
var (
	alipanInstance *AlipanService
	alipanOnce     sync.Once
)

// NewAlipanService �����������̷��񣨵���ģʽ��
func NewAlipanService(config *PanConfig) *AlipanService {
	alipanOnce.Do(func() {
		alipanInstance = &AlipanService{
			BasePanService: NewBasePanService(config),
		}

		// ���ð������̵�Ĭ������ͷ
		alipanInstance.SetHeaders(map[string]string{
			"Accept":             "application/json, text/plain, */*",
			"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"Content-Type":       "application/json",
			"Origin":             "https://www.alipan.com",
			"Priority":           "u=1, i",
			"Referer":            "https://www.alipan.com/",
			"Sec-Ch-Ua":          `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`,
			"Sec-Ch-Ua-Mobile":   "?0",
			"Sec-Ch-Ua-Platform": `"Windows"`,
			"Sec-Fetch-Dest":     "empty",
			"Sec-Fetch-Mode":     "cors",
			"Sec-Fetch-Site":     "same-site",
			"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0",
			"X-Canary":           "client=web,app=share,version=v2.3.1",
		})
	})

	// ��������
	alipanInstance.UpdateConfig(config)

	return alipanInstance
}

// GetAlipanInstance ��ȡ�������̷�����ʵ��
func GetAlipanInstance() *AlipanService {
	return NewAlipanService(nil)
}

// UpdateConfig �������ã��̰߳�ȫ��
func (a *AlipanService) UpdateConfig(config *PanConfig) {
	if config == nil {
		return
	}

	a.configMutex.Lock()
	defer a.configMutex.Unlock()

	a.config = config
}

// GetServiceType ��ȡ��������
func (a *AlipanService) GetServiceType() ServiceType {
	return Alipan
}

// Transfer ת���������
func (a *AlipanService) Transfer(shareID string) (*TransferResult, error) {
	// ��ȡ���ã��̰߳�ȫ��
	a.configMutex.RLock()
	config := a.config
	a.configMutex.RUnlock()

	fmt.Printf("��ʼ�����������̷���: %s", shareID)

	// ��ȡaccess token
	accessToken, err := a.manageAccessToken()
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡaccess_tokenʧ��: %v", err)), nil
	}

	// ����Authorizationͷ
	a.SetHeader("Authorization", "Bearer "+accessToken)

	// ��ȡ������Ϣ
	shareInfo, err := a.getAlipan1(shareID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡ������Ϣʧ��: %v", err)), nil
	}

	if config.IsType == 1 {
		// ֱ�ӷ�����Դ��Ϣ
		return SuccessResult("����ɹ�", map[string]interface{}{
			"title":    shareInfo.ShareName,
			"shareUrl": config.URL,
		}), nil
	}

	// ��ȡshare token
	shareTokenResult, err := a.getAlipan2(shareID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡshare_tokenʧ��: %v", err)), nil
	}

	// ȷ���洢·��
	toPdirFid := "root" // Ĭ�ϴ洢·�������Դ������ж�ȡ
	if config.ExpiredType == 2 {
		toPdirFid = "temp" // ��ʱ��Դ·�������Դ������ж�ȡ
	}

	// ����������������
	batchRequests := make([]map[string]interface{}, 0)
	for i, fileInfo := range shareInfo.FileInfos {
		request := map[string]interface{}{
			"body": map[string]interface{}{
				"auto_rename":       true,
				"file_id":           fileInfo.FileID,
				"share_id":          shareID,
				"to_drive_id":       "2008425230",
				"to_parent_file_id": toPdirFid,
			},
			"headers": map[string]string{
				"Content-Type": "application/json",
			},
			"id":     fmt.Sprintf("%d", i),
			"method": "POST",
			"url":    "/file/copy",
		}
		batchRequests = append(batchRequests, request)
	}

	batchData := map[string]interface{}{
		"requests": batchRequests,
		"resource": "file",
	}

	// ִ����������
	copyResult, err := a.getAlipan3(batchData, shareTokenResult.ShareToken)
	if err != nil {
		return ErrorResult(fmt.Sprintf("��������ʧ��: %v", err)), nil
	}

	// ��ȡ���ƺ���ļ�ID
	fileIDList := make([]string, 0)
	for _, response := range copyResult.Responses {
		if response.Body.Code != "" {
			return ErrorResult(fmt.Sprintf("����ʧ��: %s", response.Body.Message)), nil
		}
		fileIDList = append(fileIDList, response.Body.FileID)
	}

	// ��������
	shareData := map[string]interface{}{
		"drive_id":     "2008425230",
		"expiration":   "",
		"share_pwd":    "",
		"file_id_list": fileIDList,
	}

	shareResult, err := a.getAlipan4(shareData)
	if err != nil {
		return ErrorResult(fmt.Sprintf("��������ʧ��: %v", err)), nil
	}

	return SuccessResult("ת��ɹ�", map[string]interface{}{
		"shareUrl": shareResult.ShareURL,
		"title":    shareResult.ShareTitle,
		"fid":      shareResult.FileIDList,
	}), nil
}

// GetFiles ��ȡ�ļ��б�
func (a *AlipanService) GetFiles(pdirFid string) (*TransferResult, error) {
	// ��ȡaccess token
	accessToken, err := a.manageAccessToken()
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡaccess_tokenʧ��: %v", err)), nil
	}

	// ����Authorizationͷ
	a.SetHeader("Authorization", "Bearer "+accessToken)

	if pdirFid == "" {
		pdirFid = "root"
	}

	data := map[string]interface{}{
		"all":             false,
		"drive_id":        "2008425230",
		"fields":          "*",
		"limit":           100,
		"order_by":        "updated_at",
		"order_direction": "DESC",
		"parent_file_id":  pdirFid,
		"url_expire_sec":  14400,
	}

	respData, err := a.HTTPPost("https://api.aliyundrive.com/adrive/v3/file/list", data, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡ�ļ��б�ʧ��: %v", err)), nil
	}

	var response struct {
		Message string        `json:"message"`
		Items   []interface{} `json:"items"`
	}

	if err := json.Unmarshal(respData, &response); err != nil {
		return ErrorResult("������Ӧʧ��"), nil
	}

	if response.Message != "" {
		return ErrorResult(response.Message), nil
	}

	return SuccessResult("��ȡ�ɹ�", response.Items), nil
}

// DeleteFiles ɾ���ļ�
func (a *AlipanService) DeleteFiles(fileList []string) (*TransferResult, error) {
	// ��ȡaccess token
	accessToken, err := a.manageAccessToken()
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡaccess_tokenʧ��: %v", err)), nil
	}

	// ����Authorizationͷ
	a.SetHeader("Authorization", "Bearer "+accessToken)

	data := map[string]interface{}{
		"drive_id":     "2008425230",
		"file_id_list": fileList,
	}

	_, err = a.HTTPPost("https://api.aliyundrive.com/adrive/v3/file/delete", data, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("ɾ���ļ�ʧ��: %v", err)), nil
	}

	return SuccessResult("ɾ���ɹ�", nil), nil
}

// GetUserInfo ��ȡ�û���Ϣ
func (a *AlipanService) GetUserInfo(cookie *string) (*UserInfo, error) {
	// ����Cookie
	a.SetHeader("Cookie", *cookie)

	// ��ȡaccess token
	accessToken, err := a.manageAccessToken()
	if err != nil {
		return nil, fmt.Errorf("��ȡaccess_tokenʧ��: %v", err)
	}

	// ����Authorizationͷ
	a.SetHeader("Authorization", "Bearer "+accessToken)

	// ���ð��������û���ϢAPI
	userInfoURL := "https://api.alipan.com/v2/user/get"
	resp, err := a.HTTPGet(userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�û���Ϣʧ��: %v", err)
	}

	// ������Ӧ
	var result struct {
		Code string `json:"code"`
		Data struct {
			NickName  string `json:"nick_name"`
			Avatar    string `json:"avatar"`
			DriveInfo struct {
				TotalSize string `json:"total_size"`
				UsedSize  string `json:"used_size"`
			} `json:"drive_info"`
			VipInfo struct {
				VipStatus string `json:"vip_status"`
			} `json:"vip_info"`
		} `json:"data"`
	}

	if err := a.ParseJSONResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("�����û���Ϣʧ��: %v", err)
	}

	if result.Code != "" {
		return nil, fmt.Errorf("API���ش���: %s", result.Code)
	}

	// ת��VIP״̬
	vipStatus := result.Data.VipInfo.VipStatus == "vip"

	// ת�������ַ���Ϊ�ֽ���
	totalSizeStr := result.Data.DriveInfo.TotalSize
	usedSizeStr := result.Data.DriveInfo.UsedSize

	// ���������ַ���
	totalSizeBytes := ParseCapacityString(totalSizeStr)
	usedSizeBytes := ParseCapacityString(usedSizeStr)

	return &UserInfo{
		Username:    result.Data.NickName,
		VIPStatus:   vipStatus,
		UsedSpace:   usedSizeBytes,
		TotalSpace:  totalSizeBytes,
		ServiceType: "alipan",
	}, nil
}

// getAlipan1 ͨ������id��ȡfile_id
func (a *AlipanService) getAlipan1(shareID string) (*AlipanShareInfo, error) {
	data := map[string]interface{}{
		"share_id": shareID,
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// ��ʱ����headers
	originalHeaders := a.headers
	a.SetHeaders(headers)
	defer func() { a.headers = originalHeaders }()

	respData, err := a.HTTPPost("https://api.aliyundrive.com/adrive/v3/share_link/get_share_by_anonymous", data, nil)
	if err != nil {
		return nil, err
	}

	var result AlipanShareInfo
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetUserInfoByEntity ���� entity.Cks ��ȡ�û���Ϣ����ʵ�֣�
func (a *AlipanService) GetUserInfoByEntity(cks entity.Cks) (*UserInfo, error) {
	return nil, nil
}

// getAlipan2 ͨ������id��ȡX-Share-Token
func (a *AlipanService) getAlipan2(shareID string) (*AlipanShareToken, error) {
	data := map[string]interface{}{
		"share_id": shareID,
	}

	respData, err := a.HTTPPost("https://api.aliyundrive.com/v2/share_link/get_share_token", data, nil)
	if err != nil {
		return nil, err
	}

	var result AlipanShareToken
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// getAlipan3 ��������
func (a *AlipanService) getAlipan3(batchData map[string]interface{}, shareToken string) (*AlipanBatchResult, error) {
	// ����X-Share-Tokenͷ
	a.SetHeader("X-Share-Token", shareToken)

	respData, err := a.HTTPPost("https://api.aliyundrive.com/adrive/v4/batch", batchData, nil)
	if err != nil {
		return nil, err
	}

	var result AlipanBatchResult
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// getAlipan4 ��������
func (a *AlipanService) getAlipan4(shareData map[string]interface{}) (*AlipanShareResult, error) {
	respData, err := a.HTTPPost("https://api.aliyundrive.com/adrive/v3/share_link/create", shareData, nil)
	if err != nil {
		return nil, err
	}

	var result AlipanShareResult
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (u *AlipanService) UploadFile(localFilePath string, pdirFid string) (*TransferResult, error) {
	return ErrorResult("���������ϴ�������δʵ��"), nil
}

// Mkdir �������̴����ļ��У���δʵ�֣�
func (u *AlipanService) Mkdir(parentFid, folderName string) (string, error) {
	return "", fmt.Errorf("�������̴����ļ�����δʵ��")
}

// ShareFolder �������̷����ļ��У���δʵ�֣�
func (u *AlipanService) ShareFolder(folderFid, title string) (*PasswordResult, error) {
	return nil, fmt.Errorf("���������ļ��з�����δʵ��")
}

func (u *AlipanService) SetCKSRepository(cksRepo repo.CksRepository, entity entity.Cks) {
}

// manageAccessToken ����access token
func (a *AlipanService) manageAccessToken() (string, error) {
	if a.accessToken != "" {
		return a.accessToken, nil
	}

	// ���ļ���ȡtoken
	tokenFile := filepath.Join("config", "alipan_access_token.json")

	// ���token�ļ��Ƿ����
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		// ��ȡ�µ�access token
		return a.getNewAccessToken()
	}

	// ��ȡtoken�ļ�
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return a.getNewAccessToken()
	}

	var tokenInfo struct {
		AccessToken string    `json:"access_token"`
		ExpiresAt   time.Time `json:"expires_at"`
	}

	if err := json.Unmarshal(data, &tokenInfo); err != nil {
		return a.getNewAccessToken()
	}

	// ���token�Ƿ����
	if utils.GetCurrentTime().After(tokenInfo.ExpiresAt) {
		return a.getNewAccessToken()
	}

	a.accessToken = tokenInfo.AccessToken
	return a.accessToken, nil
}

// getNewAccessToken ��ȡ�µ�access token
func (a *AlipanService) getNewAccessToken() (string, error) {
	// ������Ҫʵ�ֻ�ȡ��token���߼�
	// ������Ҫ�û���¼����ʹ��refresh token
	return "", fmt.Errorf("��Ҫʵ�ֻ�ȡ��access token���߼�")
}

// ���尢��������صĽṹ��
type AlipanShareInfo struct {
	ShareName string `json:"share_name"`
	FileInfos []struct {
		FileID string `json:"file_id"`
	} `json:"file_infos"`
}

type AlipanShareToken struct {
	ShareToken string `json:"share_token"`
}

type AlipanBatchResult struct {
	Responses []struct {
		Body struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			FileID  string `json:"file_id"`
		} `json:"body"`
	} `json:"responses"`
}

type AlipanShareResult struct {
	ShareURL   string   `json:"share_url"`
	ShareTitle string   `json:"share_title"`
	FileIDList []string `json:"file_id_list"`
}
