package pan

import (
	"fmt"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
)

// UCService UC���̷���
type UCService struct {
	*BasePanService
}

// NewUCService ����UC���̷���
func NewUCService(config *PanConfig) *UCService {
	service := &UCService{
		BasePanService: NewBasePanService(config),
	}

	// ����UC���̵�Ĭ������ͷ
	service.SetHeaders(map[string]string{
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9",
		"Content-Type":    "application/json",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	})

	return service
}

// GetServiceType ��ȡ��������
func (u *UCService) GetServiceType() ServiceType {
	return UC
}

// Transfer ת���������
func (u *UCService) Transfer(shareID string) (*TransferResult, error) {
	// TODO: ʵ��UC����ת���߼�
	return ErrorResult("UC����ת�湦����δʵ��"), nil
}

// GetFiles ��ȡ�ļ��б�
func (u *UCService) GetFiles(pdirFid string) (*TransferResult, error) {
	// TODO: ʵ��UC�����ļ��б���ȡ
	return ErrorResult("UC�����ļ��б�������δʵ��"), nil
}

// DeleteFiles ɾ���ļ�
func (u *UCService) DeleteFiles(fileList []string) (*TransferResult, error) {
	// TODO: ʵ��UC�����ļ�ɾ��
	return ErrorResult("UC�����ļ�ɾ��������δʵ��"), nil
}

func (x *UCService) UpdateConfig(config *PanConfig) {
	if config == nil {
		return
	}
	x.config = config
	if config.Cookie != "" {
		x.SetHeader("Cookie", config.Cookie)
	}
}

// GetUserInfo ��ȡ�û���Ϣ
func (u *UCService) GetUserInfo(cookie *string) (*UserInfo, error) {
	// ����Cookie
	u.SetHeader("Cookie", *cookie)

	// ����UC�����û���ϢAPI
	userInfoURL := "https://drive.uc.cn/api/user/info"

	resp, err := u.HTTPGet(userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�û���Ϣʧ��: %v", err)
	}

	// ������Ӧ
	var result struct {
		Code int `json:"code"`
		Data struct {
			Username   string `json:"username"`
			Nickname   string `json:"nickname"`
			VipStatus  int    `json:"vip_status"`
			TotalSpace int64  `json:"total_space"`
			UsedSpace  int64  `json:"used_space"`
		} `json:"data"`
	}

	if err := u.ParseJSONResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("�����û���Ϣʧ��: %v", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API���ش���: %d", result.Code)
	}

	// ת��VIP״̬
	vipStatus := result.Data.VipStatus > 0

	// ʹ��nickname��username
	username := result.Data.Nickname
	if username == "" {
		username = result.Data.Username
	}

	return &UserInfo{
		Username:    username,
		VIPStatus:   vipStatus,
		UsedSpace:   result.Data.UsedSpace,
		TotalSpace:  result.Data.TotalSpace,
		ServiceType: "uc",
	}, nil
}

// GetUserInfoByEntity ���� entity.Cks ��ȡ�û���Ϣ����ʵ�֣�
func (u *UCService) GetUserInfoByEntity(cks entity.Cks) (*UserInfo, error) {
	return nil, nil
}

func (u *UCService) UploadFile(localFilePath string, pdirFid string) (*TransferResult, error) {
	return ErrorResult("UC�����ϴ�������δʵ��"), nil
}

// Mkdir UC���̴����ļ��У���δʵ�֣�
func (u *UCService) Mkdir(parentFid, folderName string) (string, error) {
	return "", fmt.Errorf("UC���̴����ļ�����δʵ��")
}

// ShareFolder UC���̷����ļ��У���δʵ�֣�
func (u *UCService) ShareFolder(folderFid, title string) (*PasswordResult, error) {
	return nil, fmt.Errorf("UC�����ļ��з�����δʵ��")
}

func (u *UCService) SetCKSRepository(cksRepo repo.CksRepository, entity entity.Cks) {
}
