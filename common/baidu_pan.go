package pan

import (
	"fmt"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
)

// BaiduPanService �ٶ����̷���
type BaiduPanService struct {
	*BasePanService
}

// NewBaiduPanService �����ٶ����̷���
func NewBaiduPanService(config *PanConfig) *BaiduPanService {
	service := &BaiduPanService{
		BasePanService: NewBasePanService(config),
	}

	// ���ðٶ����̵�Ĭ������ͷ
	service.SetHeaders(map[string]string{
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9",
		"Content-Type":    "application/json",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	})

	return service
}

// GetServiceType ��ȡ��������
func (b *BaiduPanService) GetServiceType() ServiceType {
	return BaiduPan
}

// Transfer ת���������
func (b *BaiduPanService) Transfer(shareID string) (*TransferResult, error) {
	// TODO: ʵ�ְٶ�����ת���߼�
	return ErrorResult("�ٶ�����ת�湦����δʵ��"), nil
}

// GetFiles ��ȡ�ļ��б�
func (b *BaiduPanService) GetFiles(pdirFid string) (*TransferResult, error) {
	// TODO: ʵ�ְٶ������ļ��б���ȡ
	return ErrorResult("�ٶ������ļ��б�������δʵ��"), nil
}

// DeleteFiles ɾ���ļ�
func (b *BaiduPanService) DeleteFiles(fileList []string) (*TransferResult, error) {
	// TODO: ʵ�ְٶ������ļ�ɾ��
	return ErrorResult("�ٶ������ļ�ɾ��������δʵ��"), nil
}

// GetUserInfo ��ȡ�û���Ϣ
func (b *BaiduPanService) GetUserInfo(cookie *string) (*UserInfo, error) {
	// ����Cookie
	b.SetHeader("Cookie", *cookie)

	// ���ðٶ������û���ϢAPI
	userInfoURL := "https://pan.baidu.com/api/gettemplatevariable"
	data := map[string]interface{}{
		"fields": "['username','uk','vip_type','vip_endtime','total_capacity','used_capacity']",
	}

	resp, err := b.HTTPPost(userInfoURL, data, nil)
	if err != nil {
		return nil, fmt.Errorf("��ȡ�û���Ϣʧ��: %v", err)
	}

	// ������Ӧ
	var result struct {
		Errno int `json:"errno"`
		Data  struct {
			Username      string `json:"username"`
			Uk            string `json:"uk"`
			VipType       int    `json:"vip_type"`
			VipEndtime    string `json:"vip_endtime"`
			TotalCapacity string `json:"total_capacity"`
			UsedCapacity  string `json:"used_capacity"`
		} `json:"data"`
	}

	if err := b.ParseJSONResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("�����û���Ϣʧ��: %v", err)
	}

	if result.Errno != 0 {
		return nil, fmt.Errorf("API���ش���: %d", result.Errno)
	}

	// ת��VIP״̬
	vipStatus := result.Data.VipType > 0

	// ���������ַ���
	totalCapacityBytes := ParseCapacityString(result.Data.TotalCapacity)
	usedCapacityBytes := ParseCapacityString(result.Data.UsedCapacity)

	return &UserInfo{
		Username:    result.Data.Username,
		VIPStatus:   vipStatus,
		UsedSpace:   usedCapacityBytes,
		TotalSpace:  totalCapacityBytes,
		ServiceType: "baidu",
	}, nil
}

// GetUserInfoByEntity ���� entity.Cks ��ȡ�û���Ϣ����ʵ�֣�
func (b *BaiduPanService) GetUserInfoByEntity(cks entity.Cks) (*UserInfo, error) {
	return nil, nil
}

func (u *BaiduPanService) UploadFile(localFilePath string, pdirFid string) (*TransferResult, error) {
	return ErrorResult("�ٶ������ϴ�������δʵ��"), nil
}

// Mkdir �ٶ����̴����ļ��У���δʵ�֣�
func (u *BaiduPanService) Mkdir(parentFid, folderName string) (string, error) {
	return "", fmt.Errorf("�ٶ����̴����ļ�����δʵ��")
}

// ShareFolder �ٶ����̷����ļ��У���δʵ�֣�
func (u *BaiduPanService) ShareFolder(folderFid, title string) (*PasswordResult, error) {
	return nil, fmt.Errorf("�ٶ������ļ��з�����δʵ��")
}

func (u *BaiduPanService) SetCKSRepository(cksRepo repo.CksRepository, entity entity.Cks) {
}

func (x *BaiduPanService) UpdateConfig(config *PanConfig) {
	if config == nil {
		return
	}
	x.config = config
	if config.Cookie != "" {
		x.SetHeader("Cookie", config.Cookie)
	}
}
