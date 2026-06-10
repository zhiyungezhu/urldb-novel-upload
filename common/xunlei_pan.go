package pan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
)

// CaptchaData �洢�����ݿ��е���֤����������
type CaptchaData struct {
	CaptchaToken string `json:"captcha_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// XunleiExtraData ���ж������ݵ�����
type XunleiTokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
	Sub          string `json:"sub"`
	TokenType    string `json:"token_type"`
	UserId       string `json:"user_id"`
}

type XunleiExtraData struct {
	Captcha     *CaptchaData              `json:"captcha,omitempty"`
	Token       *XunleiTokenData          `json:"token,omitempty"`
	Credentials *XunleiAccountCredentials `json:"credentials,omitempty"` // �˺�������Ϣ
}

type XunleiPanService struct {
	*BasePanService
	configMutex sync.RWMutex
	clientId    string
	deviceId    string
	entity      entity.Cks
	cksRepo     repo.CksRepository
	extra       XunleiExtraData // ��Ҫ���浽���ݿ��token��Ϣ
}

// ���û� API Host
func (x *XunleiPanService) apiHost(apiType string) string {
	if apiType == "user" {
		return "https://xluser-ssl.xunlei.com"
	}
	return "https://api-pan.xunlei.com"
}

func (x *XunleiPanService) setCommonHeader(req *http.Request) {
	for k, v := range x.headers {
		req.Header.Set(k, v)
	}
}

// NewXunleiPanService ����Ѹ�����̷���
func NewXunleiPanService(config *PanConfig) *XunleiPanService {
	xunleiInstance := &XunleiPanService{
		BasePanService: NewBasePanService(config),
		clientId:       "Xqp0kJBXWhwaTpB6",
		deviceId:       "925b7631473a13716b791d7f28289cad",
		extra:          XunleiExtraData{}, // Initialize extra with zero values
	}
	xunleiInstance.SetHeaders(map[string]string{
		"Accept":             "*/;",
		"Accept-Encoding":    "deflate",
		"Accept-Language":    "zh-CN,zh;q=0.9",
		"Cache-Control":      "no-cache",
		"Content-Type":       "application/json",
		"Origin":             "https://pan.xunlei.com",
		"Pragma":             "no-cache",
		"Priority":           "u=1,i",
		"Referer":            "https://pan.xunlei.com/",
		"sec-ch-ua":          `"Not;A=Brand";v="99", "Google Chrome";v="139", "Chromium";v="139"`,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
		"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
		"Authorization":      "",
		"x-captcha-token":    "",
		"x-client-id":        xunleiInstance.clientId,
		"x-device-id":        xunleiInstance.deviceId,
	})

	xunleiInstance.UpdateConfig(config)
	return xunleiInstance
}

func (x *XunleiPanService) UploadFile(localFilePath string, pdirFid string) (*TransferResult, error) {
	return ErrorResult("Ѹ�������ϴ�������δʵ��"), nil
}

// Mkdir Ѹ�����̴����ļ��У���δʵ�֣�
func (x *XunleiPanService) Mkdir(parentFid, folderName string) (string, error) {
	return "", fmt.Errorf("Ѹ�����̴����ļ�����δʵ��")
}

// ShareFolder Ѹ�����̷����ļ��У���δʵ�֣�
func (x *XunleiPanService) ShareFolder(folderFid, title string) (*PasswordResult, error) {
	return nil, fmt.Errorf("Ѹ�������ļ��з�����δʵ��")
}

// SetCKSRepository ���� CksRepository �� entity
func (x *XunleiPanService) SetCKSRepository(cksRepo repo.CksRepository, entity entity.Cks) {
	x.cksRepo = cksRepo
	x.entity = entity
	var extra XunleiExtraData

	// ����extra�ֶ�
	if x.entity.Extra != "" {
		if err := json.Unmarshal([]byte(x.entity.Extra), &extra); err != nil {
			log.Printf("���� extra ����ʧ��: %v", err)
		}
	}

	// ��ck�ֶν����˺�����
	if credentials, err := ParseCredentialsFromCk(x.entity.Ck); err == nil {
		extra.Credentials = credentials
	}

	x.extra = extra
}

// GetXunleiInstance ��ȡѸ�����̷�����ʵ��
func GetXunleiInstance() *XunleiPanService {
	return NewXunleiPanService(nil)
}

func (x *XunleiPanService) GetAccessTokenByRefreshToken(refreshToken string) (XunleiTokenData, error) {
	// ����������
	body := map[string]interface{}{
		"client_id":     x.clientId,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}

	// ���� headers���Ƴ� Authorization �� x-captcha-token��
	filteredHeaders := make(map[string]string)
	for k, v := range x.headers {
		if k != "Authorization" && k != "x-captcha-token" {
			filteredHeaders[k] = v
		}
	}

	// ���� API ��ȡ�µ� token
	resp, err := x.requestXunleiApi("https://xluser-ssl.xunlei.com/v1/auth/token", "POST", body, nil, filteredHeaders)
	if err != nil {
		return XunleiTokenData{}, fmt.Errorf("��ȡ access_token ����ʧ��: %v", err)
	}

	// ��ȷ�������� exists �ж�
	if _, exists := resp["access_token"]; exists {
		// ���������ʹֵΪ nil
	} else {
		return XunleiTokenData{}, fmt.Errorf("��ȡ access_token ����ʧ��: %v ������", "access_token")
	}

	// �������ʱ�䣨��ǰʱ�� + expires_in - 60 �뻺�壩
	currentTime := time.Now().Unix()
	expiresAt := currentTime + int64(resp["expires_in"].(float64)) - 60
	resp["expires_at"] = expiresAt
	jsonBytes, _ := json.Marshal(resp)

	var result XunleiTokenData
	json.Unmarshal(jsonBytes, &result)
	return result, nil
}

// reloginWithCredentials ʹ���˺��������µ�¼
func (x *XunleiPanService) reloginWithCredentials() (XunleiTokenData, error) {
	if x.extra.Credentials == nil {
		return XunleiTokenData{}, fmt.Errorf("���˺�������Ϣ")
	}

	tokenData, err := x.LoginWithCredentials(x.extra.Credentials.Username, x.extra.Credentials.Password)
	if err != nil {
		return XunleiTokenData{}, fmt.Errorf("�˺������¼ʧ��: %v", err)
	}

	log.Printf("�˺� %s ���µ�¼�ɹ�", x.extra.Credentials.Username)
	return tokenData, nil
}

// getAccessToken ��ȡ Access Token���ڲ����������жϡ�ˢ�¡����µ�¼�����棩
func (x *XunleiPanService) getAccessToken() (string, error) {
	// ��� Access Token �Ƿ���Ч
	currentTime := time.Now().Unix()
	if x.extra.Token != nil && x.extra.Token.AccessToken != "" && x.extra.Token.ExpiresAt > currentTime {
		return x.extra.Token.AccessToken, nil
	}

	// ����ʹ��refresh_tokenˢ��
	var newData XunleiTokenData
	var err error

	if x.extra.Token != nil && x.extra.Token.RefreshToken != "" {
		newData, err = x.GetAccessTokenByRefreshToken(x.extra.Token.RefreshToken)
		if err != nil {
			log.Printf("refresh_tokenˢ��ʧ��: %v������ʹ���˺��������µ�¼", err)

			// ���refresh_tokenʧЧ�����˺�������Ϣ���������µ�¼
			if x.extra.Credentials != nil && x.extra.Credentials.Username != "" && x.extra.Credentials.Password != "" {
				newData, err = x.reloginWithCredentials()
				if err != nil {
					return "", fmt.Errorf("���µ�¼ʧ��: %v", err)
				}
			} else {
				return "", fmt.Errorf("refresh_tokenʧЧ�����˺�������Ϣ���޷����µ�¼: %v", err)
			}
		}
	} else {
		return "", fmt.Errorf("����Ч��refresh_token")
	}

	// ����token��Ϣ
	if x.extra.Token == nil {
		x.extra.Token = &XunleiTokenData{}
	}
	x.extra.Token.AccessToken = newData.AccessToken
	x.extra.Token.RefreshToken = newData.RefreshToken
	x.extra.Token.ExpiresAt = newData.ExpiresAt
	x.extra.Token.ExpiresIn = newData.ExpiresIn
	x.extra.Token.Sub = newData.Sub
	x.extra.Token.TokenType = newData.TokenType
	x.extra.Token.UserId = newData.UserId

	// ����ck�ֶ��е�refresh_token�����������ݣ�
	x.entity.Ck = newData.RefreshToken

	// ���浽���ݿ�
	extraBytes, err := json.Marshal(x.extra)
	if err != nil {
		return "", fmt.Errorf("���л� extra ����ʧ��: %v", err)
	}
	x.entity.Extra = string(extraBytes)
	if err := x.cksRepo.UpdateWithAllFields(&x.entity); err != nil {
		return "", fmt.Errorf("���� access_token �����ݿ�ʧ��: %v", err)
	}

	return newData.AccessToken, nil
}

// getCaptchaToken ��ȡ captcha_token - ƥ�� PHP �汾
func (x *XunleiPanService) getCaptchaToken() (string, error) {
	// ��� Captcha Token �Ƿ���Ч
	currentTime := time.Now().Unix()
	if x.extra.Captcha != nil && x.extra.Captcha.CaptchaToken != "" && x.extra.Captcha.ExpiresAt > currentTime {
		return x.extra.Captcha.CaptchaToken, nil
	}

	// ����������
	body := map[string]interface{}{
		"client_id": x.clientId,
		"action":    "get:/drive/v1/share",
		"device_id": x.deviceId,
		"meta": map[string]interface{}{
			"username":       "",
			"phone_number":   "",
			"email":          "",
			"package_name":   "pan.xunlei.com",
			"client_version": "1.45.0",
			"captcha_sign":   "1.fe2108ad808a74c9ac0243309242726c",
			"timestamp":      "1645241033384",
			"user_id":        "0",
		},
	}

	captchaHeaders := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}

	// ���� API ��ȡ captcha_token
	resp, err := x.requestXunleiApi("https://xluser-ssl.xunlei.com/v1/shield/captcha/init", "POST", body, nil, captchaHeaders)
	if err != nil {
		return "", fmt.Errorf("��ȡ captcha_token ����ʧ��: %v", err)
	}

	if resp["captcha_token"] != nil && resp["captcha_token"] != "" {
		//
	} else {
		return "", fmt.Errorf("��ȡ captcha_token ʧ��: %v", resp)
	}

	// �������ʱ�䣨��ǰʱ�� + expires_in - 10 �뻺�壩
	expiresAt := currentTime + int64(resp["expires_in"].(float64)) - 10

	// ���� extra ����
	if x.extra.Captcha == nil {
		x.extra.Captcha = &CaptchaData{}
	}
	x.extra.Captcha.CaptchaToken = resp["captcha_token"].(string)
	x.extra.Captcha.ExpiresAt = expiresAt

	// ���浽���ݿ�
	extraBytes, err := json.Marshal(x.extra)
	if err != nil {
		return "", fmt.Errorf("���л� extra ����ʧ��: %v", err)
	}
	x.entity.Extra = string(extraBytes)
	if err := x.cksRepo.UpdateWithAllFields(&x.entity); err != nil {
		return "", fmt.Errorf("���� captcha_token �����ݿ�ʧ��: %v", err)
	}

	return resp["captcha_token"].(string), nil
}

// requestXunleiApi Ѹ�� API ͨ�����󷽷� - ʹ�� BasePanService ����
func (x *XunleiPanService) requestXunleiApi(url string, method string, data map[string]interface{}, queryParams map[string]string, headers map[string]string) (map[string]interface{}, error) {
	var respData []byte
	var err error

	// ����Ƿ�����֤���ʼ������
	if strings.Contains(url, "shield/captcha/init") {
		// ������֤���ʼ����ֱ�ӷ���HTTP���󣬲�ʹ��BasePanService��ʹ��sendCaptchaRequestForGeneralAPI
		return x.sendCaptchaRequestForGeneralAPI(url, data)
	}

	// �ȸ��µ�ǰ����� headers
	originalHeaders := make(map[string]string)
	for k, v := range x.headers {
		originalHeaders[k] = v
	}

	// ��ʱ��������� headers
	for k, v := range headers {
		x.SetHeader(k, v)
	}
	defer func() {
		// �ָ�ԭʼ headers
		for k, v := range originalHeaders {
			x.SetHeader(k, v)
		}
	}()

	// ���ݷ���������Ӧ�� BasePanService ����
	if method == "GET" {
		respData, err = x.HTTPGet(url, queryParams)
	} else if method == "POST" {
		respData, err = x.HTTPPost(url, data, queryParams)
	} else {
		return nil, fmt.Errorf("��֧�ֵ�HTTP����: %s", method)
	}

	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("JSON ����ʧ��: %v, raw: %s", err, string(respData))
	}

	return result, nil
}

func (x *XunleiPanService) UpdateConfig(config *PanConfig) {
	if config == nil {
		return
	}
	x.configMutex.Lock()
	defer x.configMutex.Unlock()
	x.config = config
	if config.Cookie != "" {
		x.SetHeader("Cookie", config.Cookie)
	}
}

// GetServiceType ��ȡ��������
func (x *XunleiPanService) GetServiceType() ServiceType {
	return Xunlei
}

func extractCode(url string) string {
	// ���� pwd= ��λ��
	if pwdIndex := strings.Index(url, "pwd="); pwdIndex != -1 {
		code := url[pwdIndex+4:]
		// �Ƴ� # ����������ݣ�������ڣ�
		if hashIndex := strings.Index(code, "#"); hashIndex != -1 {
			code = code[:hashIndex]
		}
		return code
	}
	return ""
}

// Transfer ת��������� - ʵ�� PanService �ӿڣ�ƥ�� XunleiPan.php ���߼�
func (x *XunleiPanService) Transfer(shareID string) (*TransferResult, error) {
	// ��ȡ���ã��̰߳�ȫ��
	x.configMutex.RLock()
	config := x.config
	x.configMutex.RUnlock()

	log.Printf("��ʼ����Ѹ�׷���: %s", shareID)

	// 1?? ��ȡ AccessToken �� CaptchaToken
	accessToken, err := x.getAccessToken()
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡaccessTokenʧ��: %v", err)), nil
	}

	captchaToken, err := x.getCaptchaToken()
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡcaptchaTokenʧ��: %v", err)), nil
	}

	// ת��ģʽ��ʵ��������ת������
	thisCode := extractCode(config.URL)

	// ��ȡ��������
	shareDetail, err := x.getShare(shareID, thisCode, accessToken, captchaToken)
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡ��������ʧ��: %v", err)), nil
	}
	if shareDetail["share_status"].(string) != "OK" {
		return ErrorResult(fmt.Sprintf("��ȡ��������ʧ��: %v", "����״̬�쳣")), nil
	}
	if shareDetail["file_num"].(string) == "0" {
		return ErrorResult(fmt.Sprintf("��ȡ��������ʧ��: %v", "�ļ��б�Ϊ��")), nil
	}

	parent_id := "" // Ĭ�ϴ洢·��

	// ����Ƿ�Ϊ����ģʽ
	if config.IsType == 1 {
		// ����ģʽ��ֱ�ӻ�ȡ������Ϣ
		urls := map[string]interface{}{
			"title":     shareDetail["title"],
			"share_url": config.URL,
			"stoken":    "",
		}
		return SuccessResult("����ɹ�", urls), nil
	}

	// files := shareDetail["files"].([]interface{})
	// fileIDs := make([]string, 0)
	// for _, file := range files {
	// 	fileMap := file.(map[string]interface{})
	// 	if fid, ok := fileMap["id"].(string); ok {
	// 		fileIDs = append(fileIDs, fid)
	// 	}
	// }

	// ���������ˣ�����򻯴�����
	// TODO: ���ӹ���ļ������߼�

	// ת����Դ
	restoreResult, err := x.getRestore(shareID, shareDetail, accessToken, captchaToken, parent_id)
	if err != nil {
		return ErrorResult(fmt.Sprintf("ת��ʧ��: %v", err)), nil
	}

	// ��ȡת��������Ϣ
	taskID := restoreResult["restore_task_id"].(string)

	// �ȴ�ת�����
	taskResp, err := x.waitForTask(taskID, accessToken, captchaToken)
	if err != nil {
		return ErrorResult(fmt.Sprintf("�ȴ�ת�����ʧ��: %v", err)), nil
	}

	// ��ȡ�������Ի�ȡ�ļ�ID
	existingFileIds := make([]string, 0)
	if params, ok2 := taskResp["params"].(map[string]interface{}); ok2 {
		if traceIds, ok3 := params["trace_file_ids"].(string); ok3 {
			traceData := make(map[string]interface{})
			json.Unmarshal([]byte(traceIds), &traceData)
			for _, fid := range traceData {
				existingFileIds = append(existingFileIds, fid.(string))
			}
		}
	}

	// ������������
	expirationDays := "-1"
	if config.ExpiredType == 2 {
		expirationDays = "2"
	}

	// ����share_id��ȡ����������
	shareResult, err := x.getSharePassword(existingFileIds, accessToken, captchaToken, expirationDays)
	if err != nil {
		return ErrorResult(fmt.Sprintf("������������ʧ��: %v", err)), nil
	}

	var fid string
	if len(existingFileIds) > 1 {
		fid = strings.Join(existingFileIds, ",")
	} else {
		fid = existingFileIds[0]
	}

	result := map[string]interface{}{
		"title":    "",
		"shareUrl": shareResult["share_url"].(string) + "?pwd=" + shareResult["pass_code"].(string),
		"code":     shareResult["pass_code"].(string),
		"fid":      fid,
	}

	return SuccessResult("ת��ɹ�", result), nil
}

// waitForTask �ȴ�������� - ʹ�� HTTPGet ����
func (x *XunleiPanService) waitForTask(taskID string, accessToken, captchaToken string) (map[string]interface{}, error) {
	maxRetries := 50
	retryDelay := 2 * time.Second

	for retryIndex := 0; retryIndex < maxRetries; retryIndex++ {
		result, err := x.getTaskStatus(taskID, retryIndex, accessToken, captchaToken)
		if err != nil {
			return nil, err
		}

		if int64(result["progress"].(float64)) == 100 { // �������
			return result, nil
		}

		time.Sleep(retryDelay)
	}

	return nil, fmt.Errorf("����ʱ")
}

// getTaskStatus ��ȡ����״̬ - ʹ�� HTTPGet ����
func (x *XunleiPanService) getTaskStatus(taskID string, retryIndex int, accessToken, captchaToken string) (map[string]interface{}, error) {
	apiURL := x.apiHost("") + "/drive/v1/tasks/" + taskID
	queryParams := map[string]string{}

	// ���� request ����� headers
	headers := map[string]string{
		"Authorization":   "Bearer " + accessToken,
		"x-captcha-token": captchaToken,
	}

	resp, err := x.requestXunleiApi(apiURL, "GET", nil, queryParams, headers)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUserInfoByEntity ���� entity.Cks ��ȡ�û���Ϣ����ʵ�֣�
func (x *XunleiPanService) GetUserInfoByEntity(cks entity.Cks) (*UserInfo, error) {
	return nil, nil
}

// getShare ��ȡ�������� - ƥ�� PHP �汾
func (x *XunleiPanService) getShare(shareID, passCode, accessToken, captchaToken string) (map[string]interface{}, error) {
	// ���� headers
	headers := make(map[string]string)
	for k, v := range x.headers {
		headers[k] = v
	}
	headers["Authorization"] = "Bearer " + accessToken
	headers["x-captcha-token"] = captchaToken

	queryParams := map[string]string{
		"share_id":        shareID,
		"pass_code":       passCode,
		"limit":           "100",
		"pass_code_token": "",
		"page_token":      "",
		"thumbnail_size":  "SIZE_SMALL",
	}

	return x.requestXunleiApi("https://api-pan.xunlei.com/drive/v1/share", "GET", nil, queryParams, headers)
}

// getRestore ת�浽���� - ƥ�� PHP �汾
func (x *XunleiPanService) getRestore(shareID string, infoData map[string]interface{}, accessToken, captchaToken, parentID string) (map[string]interface{}, error) {
	ids := make([]string, 0)
	if files, ok := infoData["files"].([]interface{}); ok {
		for _, file := range files {
			if fileMap, ok2 := file.(map[string]interface{}); ok2 {
				if id, ok3 := fileMap["id"].(string); ok3 {
					ids = append(ids, id)
				}
			}
		}
	}

	passCodeToken := ""
	if token, ok := infoData["pass_code_token"]; ok {
		if tokenStr, ok2 := token.(string); ok2 {
			passCodeToken = tokenStr
		}
	}

	data := map[string]interface{}{
		"parent_id":         parentID,
		"share_id":          shareID,
		"pass_code_token":   passCodeToken,
		"ancestor_ids":      []string{},
		"specify_parent_id": true,
		"file_ids":          ids,
	}

	headers := make(map[string]string)
	for k, v := range x.headers {
		headers[k] = v
	}
	headers["Authorization"] = "Bearer " + accessToken
	headers["x-captcha-token"] = captchaToken

	return x.requestXunleiApi("https://api-pan.xunlei.com/drive/v1/share/restore", "POST", data, nil, headers)
}

// getTasks ��ȡת������״̬ - ƥ�� PHP �汾
func (x *XunleiPanService) getTasks(taskID, accessToken, captchaToken string) (map[string]interface{}, error) {
	headers := make(map[string]string)
	for k, v := range x.headers {
		headers[k] = v
	}
	headers["Authorization"] = "Bearer " + accessToken
	headers["x-captcha-token"] = captchaToken

	return x.requestXunleiApi("https://api-pan.xunlei.com/drive/v1/tasks/"+taskID, "GET", nil, nil, headers)
}

// getSharePassword ������������ - ƥ�� PHP �汾
func (x *XunleiPanService) getSharePassword(fileIDs []string, accessToken, captchaToken, expirationDays string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"file_ids": fileIDs,
		"share_to": "copy",
		"params": map[string]interface{}{
			"subscribe_push":     "false",
			"WithPassCodeInLink": "true",
		},
		"title":           "������Դ����",
		"restore_limit":   "-1",
		"expiration_days": expirationDays,
	}

	headers := make(map[string]string)
	for k, v := range x.headers {
		headers[k] = v
	}
	headers["Authorization"] = "Bearer " + accessToken
	headers["x-captcha-token"] = captchaToken

	return x.requestXunleiApi("https://api-pan.xunlei.com/drive/v1/share", "POST", data, nil, headers)
}

// getShareInfo ��ȡ������Ϣ�����ڼ���ģʽ��
func (x *XunleiPanService) getShareInfo(shareID string) (*XLShareInfo, error) {
	// ʹ�����е� GetShareFolder ������ȡ������Ϣ
	shareDetail, err := x.GetShareFolder(shareID, "", "")
	if err != nil {
		return nil, err
	}

	// ���������Ϣ
	shareInfo := &XLShareInfo{
		ShareID: shareID,
		Title:   fmt.Sprintf("Ѹ�׷���_%s", shareID),
		Files:   make([]XLFileInfo, 0),
	}

	// �����ļ���Ϣ
	for _, file := range shareDetail.Data.Files {
		shareInfo.Files = append(shareInfo.Files, XLFileInfo{
			FileID: file.FileID,
			Name:   file.Name,
		})
	}

	return shareInfo, nil
}

// GetFiles ��ȡ�ļ��б� - ƥ�� PHP �汾�ӿڵ���
func (x *XunleiPanService) GetFiles(pdirFid string) (*TransferResult, error) {
	log.Printf("��ʼ��ȡѸ�������ļ��б���Ŀ¼ID: %s", pdirFid)

	// ��ȡ tokens
	accessToken, err := x.getAccessToken()
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡaccessTokenʧ��: %v", err)), nil
	}

	captchaToken, err := x.getCaptchaToken()
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡcaptchaTokenʧ��: %v", err)), nil
	}

	// ���� headers
	headers := make(map[string]string)
	for k, v := range x.headers {
		headers[k] = v
	}
	headers["Authorization"] = "Bearer " + accessToken
	headers["x-captcha-token"] = captchaToken

	filters := map[string]interface{}{
		"phase": map[string]interface{}{
			"eq": "PHASE_TYPE_COMPLETE",
		},
		"trashed": map[string]interface{}{
			"eq": false,
		},
	}

	filtersStr, _ := json.Marshal(filters)
	queryParams := map[string]string{
		"parent_id":      pdirFid,
		"filters":        string(filtersStr),
		"with_audit":     "true",
		"thumbnail_size": "SIZE_SMALL",
		"limit":          "50",
	}

	result, err := x.requestXunleiApi("https://api-pan.xunlei.com/drive/v1/files", "GET", nil, queryParams, headers)
	if err != nil {
		return ErrorResult(fmt.Sprintf("��ȡ�ļ��б�ʧ��: %v", err)), nil
	}

	if code, ok := result["code"].(float64); ok && code != 0 {
		return ErrorResult("��ȡ�ļ��б�ʧ��"), nil
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		if files, ok2 := data["files"]; ok2 {
			return SuccessResult("��ȡ�ɹ�", files), nil
		}
	}

	return SuccessResult("��ȡ�ɹ�", []interface{}{}), nil
}

// DeleteFiles ɾ���ļ� - ʵ�� PanService �ӿ�
func (x *XunleiPanService) DeleteFiles(fileList []string) (*TransferResult, error) {
	log.Printf("��ʼɾ��Ѹ�������ļ����ļ�����: %d", len(fileList))

	// ʹ�����е� ShareBatchDelete ����ɾ������
	result, err := x.ShareBatchDelete(fileList)
	if err != nil {
		return ErrorResult(fmt.Sprintf("ɾ���ļ�ʧ��: %v", err)), nil
	}

	if result.Code != 0 {
		return ErrorResult(fmt.Sprintf("ɾ���ļ�ʧ��: %s", result.Msg)), nil
	}

	return SuccessResult("ɾ���ɹ�", nil), nil
}

// GetUserInfo ��ȡ�û���Ϣ - ʵ�� PanService �ӿڣ�cookie ����Ϊ refresh_token���Ȼ�ȡ access_token �ٷ��� API
func (x *XunleiPanService) GetUserInfo(cookie *string) (*UserInfo, error) {
	userInfo := &UserInfo{}
	accessToken, err := x.getAccessToken()
	if err != nil {
		return nil, err
	}

	captchaToken, err := x.getCaptchaToken()
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for k, v := range x.headers {
		headers[k] = v
	}
	headers["Authorization"] = "Bearer " + accessToken
	headers["x-captcha-token"] = captchaToken

	resp, err := x.requestXunleiApi("https://api-pan.xunlei.com/drive/v1/about", "GET", nil, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�û���Ϣʧ��: %v", err)
	}
	limit := resp["quota"].(map[string]interface{})["limit"].(string)
	limitInt, _ := strconv.ParseInt(limit, 10, 64)
	used := resp["quota"].(map[string]interface{})["usage"].(string)
	usedInt, _ := strconv.ParseInt(used, 10, 64)
	userInfo.TotalSpace = limitInt
	userInfo.UsedSpace = usedInt

	// ��ȡ�û���Ϣ
	respData, err := x.requestXunleiApi(x.apiHost("user")+"/v1/user/me", "GET", nil, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�û���Ϣʧ��: %v", err)
	}

	vipInfo := respData["vip_info"].([]interface{})
	isVip := vipInfo[0].(map[string]interface{})["is_vip"].(string) != "0"

	userInfo.Username = respData["name"].(string)
	userInfo.ServiceType = x.GetServiceType().String()
	userInfo.VIPStatus = isVip
	return userInfo, nil
}

// GetShareList �ϸ���� GET + query��ʹ�� BasePanService��
func (x *XunleiPanService) GetShareList(pageToken string) (*XLShareListResp, error) {
	api := x.apiHost("") + "/drive/v1/share/list"
	queryParams := map[string]string{
		"limit":          "100",
		"thumbnail_size": "SIZE_SMALL",
	}
	if pageToken != "" {
		queryParams["page_token"] = pageToken
	}

	respData, err := x.HTTPGet(api, queryParams)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�����б�ʧ��: %v", err)
	}

	var data XLShareListResp
	if err := json.Unmarshal(respData, &data); err != nil {
		return nil, fmt.Errorf("���������б�ʧ��: %v", err)
	}
	return &data, nil
}

// FileBatchShare ����������ʹ�� BasePanService��
func (x *XunleiPanService) FileBatchShare(ids []string, needPassword bool, expirationDays int) (*XLBatchShareResp, error) {
	apiURL := x.apiHost("") + "/drive/v1/share/batch"
	body := map[string]interface{}{
		"file_ids":        ids,
		"need_password":   needPassword,
		"expiration_days": expirationDays,
	}

	respData, err := x.HTTPPost(apiURL, body, nil)
	if err != nil {
		return nil, fmt.Errorf("��������ʧ��: %v", err)
	}

	var data XLBatchShareResp
	if err := json.Unmarshal(respData, &data); err != nil {
		return nil, fmt.Errorf("����������Ӧʧ��: %v", err)
	}
	return &data, nil
}

// ShareBatchDelete ȡ��������ʹ�� BasePanService��
func (x *XunleiPanService) ShareBatchDelete(ids []string) (*XLCommonResp, error) {
	apiURL := x.apiHost("") + "/drive/v1/share/batch/delete"
	body := map[string]interface{}{
		"share_ids": ids,
	}

	respData, err := x.HTTPPost(apiURL, body, nil)
	if err != nil {
		return nil, fmt.Errorf("ɾ������ʧ��: %v", err)
	}

	var data XLCommonResp
	if err := json.Unmarshal(respData, &data); err != nil {
		return nil, fmt.Errorf("����ɾ����Ӧʧ��: %v", err)
	}
	return &data, nil
}

// GetShareFolder ��ȡ�������ݣ�ʹ�� BasePanService��
func (x *XunleiPanService) GetShareFolder(shareID, passCodeToken, parentID string) (*XLShareFolderResp, error) {
	apiURL := x.apiHost("") + "/drive/v1/share/detail"
	body := map[string]interface{}{
		"share_id":        shareID,
		"pass_code_token": passCodeToken,
		"parent_id":       parentID,
		"limit":           100,
		"thumbnail_size":  "SIZE_LARGE",
		"order":           "6",
	}

	respData, err := x.HTTPPost(apiURL, body, nil)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�����ļ���ʧ��: %v", err)
	}

	var data XLShareFolderResp
	if err := json.Unmarshal(respData, &data); err != nil {
		return nil, fmt.Errorf("���������ļ���ʧ��: %v", err)
	}
	return &data, nil
}

// Restore ת�棨ʹ�� BasePanService��
func (x *XunleiPanService) Restore(shareID, passCodeToken string, fileIDs []string) (*XLRestoreResp, error) {
	apiURL := x.apiHost("") + "/drive/v1/share/restore"
	body := map[string]interface{}{
		"share_id":          shareID,
		"pass_code_token":   passCodeToken,
		"file_ids":          fileIDs,
		"folder_type":       "NORMAL",
		"specify_parent_id": true,
		"parent_id":         "",
	}

	respData, err := x.HTTPPost(apiURL, body, nil)
	if err != nil {
		return nil, fmt.Errorf("ת��ʧ��: %v", err)
	}

	var data XLRestoreResp
	if err := json.Unmarshal(respData, &data); err != nil {
		return nil, fmt.Errorf("����ת����Ӧʧ��: %v", err)
	}
	return &data, nil
}

// sendCaptchaRequestForGeneralAPI ������֤������ - ���ڷǵ�¼��������֤������
func (x *XunleiPanService) sendCaptchaRequestForGeneralAPI(url string, data map[string]interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	log.Printf("������֤������URL: %s", url)
	log.Printf("������֤����������: %s", string(jsonData))

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}

	// ��������ͷ
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("X-Client-Id", x.clientId)
	req.Header.Set("X-Device-Id", x.deviceId)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("��֤����Ӧ״̬��: %d", resp.StatusCode)
	log.Printf("��֤����Ӧ����: %s", string(body))

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON ����ʧ��: %v, raw: %s", err, string(body))
	}

	log.Printf("���������Ӧ: %+v", result)
	return result, nil
}

// �ṹ����ȫ���� xunleix
type XLShareListResp struct {
	Data struct {
		List []struct {
			ShareID string `json:"share_id"`
			Title   string `json:"title"`
		} `json:"list"`
	} `json:"data"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type XLBatchShareResp struct {
	Data struct {
		ShareURL string `json:"share_url"`
	} `json:"data"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type XLCommonResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type XLShareFolderResp struct {
	Data struct {
		Files []struct {
			FileID string `json:"file_id"`
			Name   string `json:"name"`
		} `json:"files"`
	} `json:"data"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type XLRestoreResp struct {
	Data struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// ���������ṹ��
type XLShareInfo struct {
	ShareID string       `json:"share_id"`
	Title   string       `json:"title"`
	Files   []XLFileInfo `json:"files"`
}

type XLFileInfo struct {
	FileID string `json:"file_id"`
	Name   string `json:"name"`
}

type XLTaskResult struct {
	Status int    `json:"status"`
	TaskID string `json:"task_id"`
	Data   struct {
		ShareID string `json:"share_id"`
	} `json:"data"`
}