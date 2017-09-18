package imguploader

// 存储相关功能的引入包只有这两个，后面不再赘述
import (
	"context"

	"fmt"
	"strings"

	"github.com/masami10/grafana/pkg/log"
	"github.com/qiniu/api.v7/auth/qbox"
	"github.com/qiniu/api.v7/storage"
)

type QiniuUploader struct {
	region       string
	bucket       string
	secretKey    string
	accessKey    string
	publicDomain string
	cfg          *storage.Config
	log          log.Logger
}

// 自定义返回值结构体
type QiniuPutRet struct {
	Key    string
	Hash   string
	Fsize  int
	Bucket string
}

func NewQiniuImageUploader(bucket string, accessKey string, secretKey string, domain string, zone string) (*QiniuUploader, error) {
	cfg := storage.Config{}
	// 空间对应的机房
	switch zone {
	case "Huadong":
		cfg.Zone = &storage.ZoneHuadong
	case "Huabei":
		cfg.Zone = &storage.ZoneHuabei
	case "Huanan":
		cfg.Zone = &storage.ZoneHuanan
	case "Beimei":
		cfg.Zone = &storage.ZoneBeimei
	default:
		return nil, fmt.Errorf("Could not find Zone information in Qiniu Provider.")
	}
	// 是否使用https域名
	cfg.UseHTTPS = false
	// 上传是否使用CDN上传加速
	cfg.UseCdnDomains = false

	return &QiniuUploader{
		bucket:       bucket,
		accessKey:    accessKey,
		secretKey:    secretKey,
		publicDomain: domain,
		log:          log.New("qiniuuploader"),
		cfg:          &cfg,
	}, nil
}

func (u *QiniuUploader) Upload(imageDiskPath string) (string, error) {

	localFile := imageDiskPath
	bucket := u.bucket
	putPolicy := storage.PutPolicy{
		Scope:      bucket,
		ReturnBody: `{"key":"$(key)","hash":"$(etag)","fsize":$(fsize),"bucket":"$(bucket)"}`,
		//FileType: 1, //设定为低频存储
	}
	mac := qbox.NewMac(u.accessKey, u.secretKey)
	upToken := putPolicy.UploadToken(mac)

	formUploader := storage.NewFormUploader(u.cfg)
	ret := QiniuPutRet{}
	// 可选配置
	//putExtra := storage.PutExtra{
	//	Params: map[string]string{
	//		"x:name": "github logo",
	//	},
	//}

	//以返回的hash数值作为Key传递
	err := formUploader.PutFileWithoutKey(context.Background(), &ret, upToken, localFile, nil)
	if err != nil {
		return "", err
	}

	var domain = u.publicDomain
	if strings.HasPrefix(domain, "https://") != true {
		domain = "https://" + u.publicDomain
	}
	fileUrl := storage.MakePublicURL(domain, ret.Hash) // urlencode 解决URL兼容性问题
	return fileUrl, nil

}
