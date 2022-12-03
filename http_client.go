package rod_helper

import (
	"crypto/tls"
	"github.com/go-resty/resty/v2"
	"net/http"
	"time"
)

// NewHttpClient 新建一个 resty 的对象
func NewHttpClient(opt *HttpClientOptions) (*resty.Client, error) {
	//const defUserAgent = "Mozilla/5.0 (Macintosh; U; Intel Mac OS X 10_6_8; en-us) AppleWebKit/534.50 (KHTML, like Gecko) Version/5.1 Safari/534.50"
	//const defUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.77 Safari/537.36 Edg/91.0.864.41"

	var UserAgent string
	// ------------------------------------------------
	// 随机的 Browser
	UserAgent = RandomUserAgent()
	// ------------------------------------------------
	httpClient := resty.New().SetTransport(&http.Transport{
		DisableKeepAlives:   true,
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
	})
	httpClient.SetTimeout(opt.HTMLTimeOut)
	httpClient.SetRetryCount(1)
	// ------------------------------------------------
	// 设置 Referer
	if len(opt.Referer) > 0 {
		httpClient.SetHeader("Referer", opt.Referer)
	}
	// ------------------------------------------------
	// 设置 Header
	httpClient.SetHeaders(map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   UserAgent,
	})
	// ------------------------------------------------
	// 不要求安全链接
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	// ------------------------------------------------
	// http 代理
	if opt.HttpProxyUrl != "" {
		httpClient.SetProxy(opt.HttpProxyUrl)
	} else {
		httpClient.RemoveProxy()
	}

	return httpClient, nil
}

type HttpClientOptions struct {
	TmpRootFolder string
	HTMLTimeOut   time.Duration
	HttpProxyUrl  string
	Referer       string
}

func NewHttpClientOptions(tmpRootFolder string, HTMLTimeOut time.Duration, httpProxyUrl string, referer string) *HttpClientOptions {
	return &HttpClientOptions{
		TmpRootFolder: tmpRootFolder,
		HTMLTimeOut:   HTMLTimeOut,
		HttpProxyUrl:  httpProxyUrl,
		Referer:       referer,
	}
}
