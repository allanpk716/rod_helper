package rod_helper

import (
	"crypto/tls"
	"github.com/go-resty/resty/v2"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func NewHttpClient(proxyUrl string, timeOut time.Duration) *resty.Client {

	UserAgent := RandomUserAgent(true)
	httpClient := resty.New().SetTransport(&http.Transport{
		DisableKeepAlives:   true,
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
	})
	httpClient.SetTimeout(timeOut)
	// 设置 Header
	httpClient.SetHeaders(map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   UserAgent,
	})
	// 不要求安全链接
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	// 设置代理
	if proxyUrl != "" {
		httpClient.SetProxy(proxyUrl)
	} else {
		httpClient.RemoveProxy()
	}

	return httpClient
}

// IsDir 存在且是文件夹
func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// IsFile 存在且是文件
func IsFile(filePath string) bool {
	s, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

// WriteFile 写文件
func WriteFile(desFileFPath string, bytes []byte) error {
	var err error
	nowDesPath := desFileFPath
	if filepath.IsAbs(nowDesPath) == false {
		nowDesPath, err = filepath.Abs(nowDesPath)
		if err != nil {
			return err
		}
	}
	// 创建对应的目录
	nowDirPath := filepath.Dir(nowDesPath)
	err = os.MkdirAll(nowDirPath, os.ModePerm)
	if err != nil {
		return err
	}
	file, err := os.Create(nowDesPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = file.Write(bytes)
	if err != nil {
		return err
	}

	return nil
}
