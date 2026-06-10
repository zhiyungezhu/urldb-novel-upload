??????package pan

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/zhiyungezhu/urldb-novel-upload/db/entity"
	"github.com/zhiyungezhu/urldb-novel-upload/db/repo"
)

// ServiceType �������̷�������
type ServiceType int

const (
	Quark ServiceType = iota
	Alipan
	BaiduPan
	UC
	NotFound
	Xunlei
	Tianyi
	Pan123
	Pan115
)

// String ���ط������͵��ַ�����ʾ
func (s ServiceType) String() string {
	switch s {
	case Quark:
		return "quark"
	case Alipan:
		return "alipan"
	case BaiduPan:
		return "baidu"
	case UC:
		return "uc"
	case Xunlei:
		return "xunlei"
	case Tianyi:
		return "tianyi"
	case Pan123:
		return "123pan"
	case Pan115:
		return "115"
	default:
		return "unknown"
	}
}

// PanConfig ��������
type PanConfig struct {
	URL         string `json:"url"`
	Code        string `json:"code"`
	IsType      int    `json:"isType"`      // 0: ת�沢���������Դ��Ϣ, 1: ֱ�ӻ�ȡ��Դ��Ϣ
	ExpiredType int    `json:"expiredType"` // 1: ��������, 2: ��ʱ
	AdFid       string `json:"adFid"`       // ���ר�� - ����ʱ��������ļ���fid
	Stoken      string `json:"stoken"`
	Cookie      string `json:"cookie"`
}

// TransferResult ת����
type TransferResult struct {
	Success  bool        `json:"success"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
	ShareURL string      `json:"shareUrl,omitempty"`
	Title    string      `json:"title,omitempty"`
	Fid      string      `json:"fid,omitempty"`
}

// UserInfo �û���Ϣ�ṹ��
type UserInfo struct {
	Username    string `json:"username"`    // �û���
	VIPStatus   bool   `json:"vipStatus"`   // VIP״̬
	UsedSpace   int64  `json:"usedSpace"`   // ��ʹ�ÿռ�
	TotalSpace  int64  `json:"totalSpace"`  // �ܿռ�
	ServiceType string `json:"serviceType"` // ��������
	ExtraData   string `json:"extraData"`   // ������Ϣ
}

// PanService ���̷���ӿ�
type PanService interface {
	// Transfer ת���������
	Transfer(shareID string) (*TransferResult, error)

	// GetFiles ��ȡ�ļ��б�
	GetFiles(pdirFid string) (*TransferResult, error)

	// DeleteFiles ɾ���ļ�
	DeleteFiles(fileList []string) (*TransferResult, error)

	// GetUserInfo ��ȡ�û���Ϣ
	GetUserInfo(ck *string) (*UserInfo, error)

	// GetServiceType ��ȡ��������
	GetServiceType() ServiceType

	// UploadFile �ϴ������ļ�������
	// localFilePath: �����ļ�·��, pdirFid: Ŀ��Ŀ¼ID�����ַ���=��Ŀ¼��
	UploadFile(localFilePath string, pdirFid string) (*TransferResult, error)

	// Mkdir �����ļ���
	// parentFid: ��Ŀ¼ID�����ַ���=��Ŀ¼��, folderName: �ļ�������
	// �������ļ��е� fid
	Mkdir(parentFid, folderName string) (string, error)

	// ShareFolder �����ļ��У������ļ����������ļ���
	// folderFid: �ļ���ID, title: ��������
	// ���ط������Ӻ���ȡ��
	ShareFolder(folderFid, title string) (*PasswordResult, error)

	SetCKSRepository(cksRepo repo.CksRepository, entity entity.Cks)

	UpdateConfig(config *PanConfig)
}

// PanFactory ���̹���
type PanFactory struct{}

// ������ر���
var (
	instance *PanFactory
	once     sync.Once
)

// NewPanFactory �������̹���ʵ��������ģʽ��
func NewPanFactory() *PanFactory {
	once.Do(func() {
		instance = &PanFactory{}
	})
	return instance
}

// GetInstance ��ȡ����ʵ�����Ƽ�ʹ�ã�
func GetInstance() *PanFactory {
	return NewPanFactory()
}

// CreatePanService ����URL������Ӧ�����̷���
func (f *PanFactory) CreatePanService(url string, config *PanConfig) (PanService, error) {
	serviceType := ExtractServiceType(url)

	switch serviceType {
	case Quark:
		return NewQuarkPanService(config), nil
	case Alipan:
		return NewAlipanService(config), nil
	case BaiduPan:
		return NewBaiduPanService(config), nil
	case UC:
		return NewUCService(config), nil
	case Xunlei:
		return NewXunleiPanService(config), nil
	default:
		return nil, fmt.Errorf("��֧�ֵķ�������: %s", url)
	}
}

// CreatePanServiceByType ���ݷ������ʹ�����Ӧ�����̷���
func (f *PanFactory) CreatePanServiceByType(serviceType ServiceType, config *PanConfig) (PanService, error) {
	switch serviceType {
	case Quark:
		return NewQuarkPanService(config), nil
	case Alipan:
		return NewAlipanService(config), nil
	case BaiduPan:
		return NewBaiduPanService(config), nil
	case UC:
		return NewUCService(config), nil
	case Xunlei:
		return NewXunleiPanService(config), nil
	// case Tianyi:
	// 	return NewTianyiService(config), nil
	default:
		return nil, fmt.Errorf("��֧�ֵķ�������: %d", serviceType)
	}
}

// GetQuarkService ��ȡ������̷�����
func (f *PanFactory) GetQuarkService(config *PanConfig) PanService {
	service := NewQuarkPanService(config)
	return service
}

// GetAlipanService ��ȡ�������̷�����
func (f *PanFactory) GetAlipanService(config *PanConfig) PanService {
	service := NewAlipanService(config)
	return service
}

// GetBaiduService ��ȡ�ٶ����̷�����
func (f *PanFactory) GetBaiduService(config *PanConfig) PanService {
	service := NewBaiduPanService(config)
	return service
}

// GetUCService ��ȡUC���̷�����
func (f *PanFactory) GetUCService(config *PanConfig) PanService {
	service := NewUCService(config)
	return service
}

// GetXunleiService ��ȡѸ�����̷�����
func (f *PanFactory) GetXunleiService(config *PanConfig) PanService {
	service := NewXunleiPanService(config)
	return service
}

// ExtractServiceType ��URL����ȡ��������
func ExtractServiceType(url string) ServiceType {
	url = strings.ToLower(url)

	// "https://www.123pan.com/s/i4uaTd-WHn0", // ��������
	// "https://www.123912.com/s/U8f2Td-ZeOX",
	// "https://www.123684.coms/u9izjv-k3uWv",
	// "https://www.123pan.com/s/A6cA-AKH11", // ����������

	patterns := map[string]ServiceType{
		"pan.quark.cn":        Quark,
		"www.alipan.com":      Alipan,
		"www.aliyundrive.com": Alipan,
		"pan.baidu.com":       BaiduPan,
		"drive.uc.cn":         UC,
		"fast.uc.cn":          UC,
		"pan.xunlei.com":      Xunlei,
		"cloud.189.cn":        Tianyi,
		"www.123pan.com":      Pan123,
		"www.123912.com":      Pan123,
		"www.123684.com":      Pan123,
		"www.123865.com":      Pan123,
		"www.123685.com":      Pan123,
		"123pan.com":          Pan123,
		"share.123pan.cn":    Pan123,
		"115cdn.com":          Pan115,
		"anxia.com":           Pan115,
		"115.com/":            Pan115,
	}

	for pattern, serviceType := range patterns {
		if strings.Contains(url, pattern) {
			return serviceType
		}
	}

	return NotFound
}

// ExtractShareId ��URL����ȡ����ID
func ExtractShareId(url string) (string, ServiceType) {
	// ����entry����
	if strings.Contains(url, "?entry=") {
		url = strings.Split(url, "?entry=")[0]
	}

	// ��ȡ����ID
	shareID := ""
	substring := -1

	if index := strings.Index(url, "/s/"); index != -1 {
		substring = index + 3
	} else if index := strings.Index(url, "/123pan/"); index != -1 {
		substring = index + 8
	} else if index := strings.Index(url, "/t/"); index != -1 {
		substring = index + 3
	} else if index := strings.Index(url, "/web/share?code="); index != -1 {
		substring = index + 16
	} else if index := strings.Index(url, "/p/"); index != -1 {
		substring = index + 3
	}

	if substring == -1 {
		return "", NotFound
	}

	shareID = url[substring:]

	// ȥ�����ܵ�ê��
	if hashIndex := strings.Index(shareID, "?"); hashIndex != -1 {
		shareID = shareID[:hashIndex]
	}
	if hashIndex := strings.Index(shareID, "#"); hashIndex != -1 {
		shareID = shareID[:hashIndex]
	}

	serviceType := ExtractServiceType(url)
	return shareID, serviceType
}

// SuccessResult �����ɹ����
func SuccessResult(message string, data interface{}) *TransferResult {
	return &TransferResult{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// ErrorResult ����������
func ErrorResult(message string) *TransferResult {
	return &TransferResult{
		Success: false,
		Message: message,
	}
}

// ParseCapacityString ���������ַ���Ϊ�ֽ���
func ParseCapacityString(capacityStr string) int64 {
	if capacityStr == "" {
		return 0
	}

	// �Ƴ��ո�ת��ΪСд
	capacityStr = strings.TrimSpace(strings.ToLower(capacityStr))

	var multiplier int64 = 1
	if strings.Contains(capacityStr, "gb") {
		multiplier = 1024 * 1024 * 1024
		capacityStr = strings.Replace(capacityStr, "gb", "", -1)
	} else if strings.Contains(capacityStr, "mb") {
		multiplier = 1024 * 1024
		capacityStr = strings.Replace(capacityStr, "mb", "", -1)
	} else if strings.Contains(capacityStr, "kb") {
		multiplier = 1024
		capacityStr = strings.Replace(capacityStr, "kb", "", -1)
	} else if strings.Contains(capacityStr, "b") {
		capacityStr = strings.Replace(capacityStr, "b", "", -1)
	}

	// ��������
	capacityStr = strings.TrimSpace(capacityStr)
	if capacityStr == "" {
		return 0
	}

	// ���Խ���������
	if strings.Contains(capacityStr, ".") {
		if val, err := strconv.ParseFloat(capacityStr, 64); err == nil {
			return int64(val * float64(multiplier))
		}
	} else {
		if val, err := strconv.ParseInt(capacityStr, 10, 64); err == nil {
			return val * multiplier
		}
	}

	return 0
}
