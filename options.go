package rod_helper

import (
	"github.com/sirupsen/logrus"
)

type BrowserOptions struct {
	Log                  *logrus.Logger // 日志
	loadAdblock          bool           // 是否加载 adblock
	preLoadUrl           string         // 预加载的url
	xrayPoolUrl          string         // xray pool url
	xrayPoolPort         string         // xray pool port
	browserInstanceCount int            // 浏览器最大的实例，xrayPoolUrl 有值的时候生效，用于爬虫。因为每启动一个实例就试用一个固定的代理，所以需要多个才行
	browserFPath         string         // 浏览器的路径
}

func NewBrowserOptions(log *logrus.Logger, loadAdblock bool) *BrowserOptions {
	return &BrowserOptions{Log: log, loadAdblock: loadAdblock, browserInstanceCount: 1}
}

func (r *BrowserOptions) SetPreLoadUrl(url string) {
	r.preLoadUrl = url
}
func (r *BrowserOptions) PreLoadUrl() string {
	return r.preLoadUrl
}

// SetXrayPoolUrl 127.0.0.1
func (r *BrowserOptions) SetXrayPoolUrl(xrayUrl string) {
	r.xrayPoolUrl = xrayUrl
}

// XrayPoolUrl 127.0.0.1
func (r *BrowserOptions) XrayPoolUrl() string {
	return r.xrayPoolUrl
}

// SetXrayPoolPort 19038
func (r *BrowserOptions) SetXrayPoolPort(xrayPort string) {
	r.xrayPoolPort = xrayPort
}

// XrayPoolPort 19038
func (r *BrowserOptions) XrayPoolPort() string {
	return r.xrayPoolPort
}

func (r *BrowserOptions) SetBrowserInstanceCount(count int) {
	r.browserInstanceCount = count
}
func (r *BrowserOptions) BrowserInstanceCount() int {
	return r.browserInstanceCount
}

func (r *BrowserOptions) LoadAdblock() bool {
	return r.loadAdblock
}

func (r *BrowserOptions) BrowserFPath() string {
	return r.browserFPath
}

func (r *BrowserOptions) SetBrowserFPath(path string) {
	r.browserFPath = path
}
