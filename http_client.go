package rod_helper

import (
	"crypto/tls"
	"github.com/go-resty/resty/v2"
	"net/http"
	"net/url"
	"time"
)

// NewHttpClient 新建一个 resty 的对象
func NewHttpClient(opt *HttpClientOptions) (*resty.Client, error) {

	var UserAgent string
	// ------------------------------------------------
	// 随机的 Browser
	UserAgent = RandomUserAgent()
	// ------------------------------------------------
	httpClient := resty.New()
	httpClient.SetTimeout(opt.htmlTimeOut)
	httpClient.SetRetryCount(1)
	// ------------------------------------------------
	// 设置 Referer
	if len(opt.Referer()) > 0 {
		httpClient.SetHeader("Referer", opt.Referer())
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
	proxyType, proxyUrl := opt.ProxyUrl()
	switch proxyType {
	case Http:
		httpClient.SetProxy(proxyUrl)
	case Socks5:
		proxy := func(_ *http.Request) (*url.URL, error) {
			return url.Parse(proxyUrl)
		}
		httpTransport := &http.Transport{
			Proxy:               proxy,
			DisableKeepAlives:   true,
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 1000,
		}
		httpClient.SetTransport(httpTransport)
	default:
		httpClient.RemoveProxy()
	}

	return httpClient, nil
}

type HttpClientOptions struct {
	htmlTimeOut    time.Duration
	proxyType      ProxyType
	httpProxyUrl   string
	socks5ProxyUrl string
	referer        string
}

func NewHttpClientOptions(HTMLTimeOut time.Duration) *HttpClientOptions {
	return &HttpClientOptions{
		htmlTimeOut: HTMLTimeOut,
	}
}

func (h *HttpClientOptions) HtmlTimeOut() time.Duration {
	return h.htmlTimeOut
}

// SetHttpProxy 输入这样的代理连接：http://127.0.0.1:9150
func (h *HttpClientOptions) SetHttpProxy(SetHttpProxy string) {
	h.proxyType = Http
	h.httpProxyUrl = SetHttpProxy
}

// SetSocks5Proxy 输入这样的代理连接：socks5://127.0.0.1:9150
func (h *HttpClientOptions) SetSocks5Proxy(SetSock5Proxy string) {
	h.proxyType = Socks5
	h.socks5ProxyUrl = SetSock5Proxy
}

func (h *HttpClientOptions) ProxyUrl() (ProxyType, string) {
	switch h.proxyType {
	case None:
		return None, ""
	case Http:
		return Http, h.httpProxyUrl
	case Socks5:
		return Socks5, h.socks5ProxyUrl
	}
	return None, ""
}

func (h *HttpClientOptions) SetReferer(referer string) {
	h.referer = referer
}

func (h *HttpClientOptions) Referer() string {
	return h.referer
}

type ProxyType int

const (
	None ProxyType = iota + 1
	Http
	Socks5
)
