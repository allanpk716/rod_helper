package rod_helper

import (
	_ "embed"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-rod/rod"
)

type Browser struct {
	log             *logrus.Logger
	rodOptions      *BrowserOptions     // 参数
	multiBrowser    []*rod.Browser      // 多浏览器实例
	browserIndex    int                 // 当前使用的浏览器的索引
	browserLocker   sync.Mutex          // 浏览器的锁
	httpProxyIndex  int                 // 当前使用的 http 代理的索引
	httpProxyLocker sync.Mutex          // http 代理的锁
	LbHttpUrl       string              // 负载均衡的 http proxy url
	LBPort          int                 //负载均衡 http 端口
	proxyInfos      []XrayPoolProxyInfo // XrayPool 中的代理信息
}

// NewMultiBrowser 面向与爬虫的时候使用 Browser
func NewMultiBrowser(browserOptions *BrowserOptions) *Browser {

	// 从配置中，判断 XrayPool 是否启动
	if browserOptions.XrayPoolUrl() == "" {
		browserOptions.Log.Errorf("XrayPoolUrl is empty")
		return nil
	}
	if browserOptions.XrayPoolPort() == "" {
		browserOptions.Log.Errorf("XrayPoolPort is empty")
		return nil
	}
	// 尝试从本地的 XrayPoolUrl 获取 代理信息
	httpClient := resty.New().SetTransport(&http.Transport{
		DisableKeepAlives:   true,
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
	})

	var proxyResult ProxyResult
	_, err := httpClient.R().
		SetResult(&proxyResult).
		Get(httpPrefix +
			browserOptions.XrayPoolUrl() +
			":" +
			browserOptions.XrayPoolPort() +
			"/v1/proxy_list")
	if err != nil {
		browserOptions.Log.Error(errors.New("Get error:" + err.Error()))
		return nil
	}

	if proxyResult.Status == "stopped" || len(proxyResult.OpenResultList) == 0 {
		browserOptions.Log.Error("XrayPool Not Started!")
		return nil
	}

	b := &Browser{
		log:          browserOptions.Log,
		rodOptions:   browserOptions,
		multiBrowser: make([]*rod.Browser, 0),
		proxyInfos:   make([]XrayPoolProxyInfo, 0),
	}

	for index, result := range proxyResult.OpenResultList {

		tmpProxyInfos := XrayPoolProxyInfo{
			Index:           index,
			Name:            result.Name,
			ProtoModel:      result.ProtoModel,
			HttpUrl:         httpPrefix + browserOptions.XrayPoolUrl() + ":" + strconv.Itoa(result.HttpPort),
			SocksUrl:        socksPrefix + browserOptions.XrayPoolUrl() + ":" + strconv.Itoa(result.SocksPort),
			skipAccessTime:  0,
			lastAccessTime:  0,
			accessTimeLines: make([]int64, 0),
		}
		b.proxyInfos = append(b.proxyInfos, tmpProxyInfos)
	}
	b.LBPort = proxyResult.LBPort

	b.LbHttpUrl = fmt.Sprintf(httpPrefix + browserOptions.XrayPoolUrl() + ":" + strconv.Itoa(b.LBPort))
	for i := 0; i < browserOptions.BrowserInstanceCount(); i++ {

		oneBrowser, err := NewBrowserBase(browserOptions.BrowserFPath(), b.LbHttpUrl, browserOptions.LoadAdblock())
		if err != nil {
			b.log.Error(errors.New("NewBrowserBase error:" + err.Error()))
			return nil
		}
		b.multiBrowser = append(b.multiBrowser, oneBrowser)
	}

	return b
}

// GetOptions 获取设置的参数
func (b *Browser) GetOptions() *BrowserOptions {
	return b.rodOptions
}

// GetLBBrowser 这里获取到的 Browser 使用的代理是负载均衡的代理
func (b *Browser) GetLBBrowser() *rod.Browser {

	b.browserLocker.Lock()
	defer func() {
		b.browserIndex++
		b.browserLocker.Unlock()
	}()

	if b.browserIndex >= len(b.multiBrowser) {
		b.browserIndex = 0
	}

	return b.multiBrowser[b.browserIndex]
}

// GetOneProxyInfo 轮询获取一个代理实例
func (b *Browser) GetOneProxyInfo() (XrayPoolProxyInfo, error) {

	b.httpProxyLocker.Lock()
	nowUnixTime := time.Now().Unix()
	defer func() {
		// 记录最后一次获取这个 Index ProxyInfo 的 UnixTime
		b.proxyInfos[b.httpProxyIndex].lastAccessTime = nowUnixTime
		b.proxyInfos[b.httpProxyIndex].accessTimeLines = append(b.proxyInfos[b.httpProxyIndex].accessTimeLines, b.proxyInfos[b.httpProxyIndex].lastAccessTime)
		// 下一个节点
		b.httpProxyIndex++
		if b.httpProxyIndex >= len(b.proxyInfos) {
			b.httpProxyIndex = 0
		}
		b.httpProxyLocker.Unlock()
	}()

	if len(b.proxyInfos) < 1 {
		return XrayPoolProxyInfo{}, ErrProxyInfosIsEmpty
	}

	if b.proxyInfos[b.httpProxyIndex].skipAccessTime > nowUnixTime {
		// 这个节点需要跳过
		return b.proxyInfos[b.httpProxyIndex], ErrSkipAccessTime
	}

	return b.proxyInfos[b.httpProxyIndex], nil
}

func (b *Browser) GetAccessTimeLines(index int) ([]int64, error) {
	b.httpProxyLocker.Lock()
	defer func() {
		b.httpProxyLocker.Unlock()
	}()

	if len(b.proxyInfos) < 1 {
		return nil, ErrProxyInfosIsEmpty
	}

	if index >= len(b.proxyInfos) {
		return nil, ErrIndexIsOutOfRange
	}

	return b.proxyInfos[index].accessTimeLines, nil
}

func (b *Browser) ClearAccessTimeLines(index int) error {
	b.httpProxyLocker.Lock()
	defer func() {
		b.httpProxyLocker.Unlock()
	}()

	if len(b.proxyInfos) < 1 {
		return ErrProxyInfosIsEmpty
	}

	if index >= len(b.proxyInfos) {
		return ErrIndexIsOutOfRange
	}

	b.proxyInfos[index].accessTimeLines = make([]int64, 0)

	return nil
}

// SetProxyNodeSkipByTime 设置这个节点，等待多少秒之后才可以被再次使用，仅仅针对 GetOneProxyInfo 有效
func (b *Browser) SetProxyNodeSkipByTime(index int, targetSkipTime int64) error {
	b.httpProxyLocker.Lock()
	defer func() {
		b.httpProxyLocker.Unlock()
	}()

	if len(b.proxyInfos) < 1 {
		return ErrProxyInfosIsEmpty
	}

	if index >= len(b.proxyInfos) {
		return ErrIndexIsOutOfRange
	}

	b.proxyInfos[index].skipAccessTime = targetSkipTime
	return nil
}

// NewBrowser 每次新建一个 Browser ，使用 HttpProxy 列表中的一个作为代理
func (b *Browser) NewBrowser() (*rod.Browser, error) {

	b.httpProxyLocker.Lock()
	defer func() {
		b.httpProxyIndex++
		b.httpProxyLocker.Unlock()
	}()

	if len(b.proxyInfos) < 1 {
		return nil, ErrProxyInfosIsEmpty
	}

	if b.httpProxyIndex >= len(b.proxyInfos) {
		b.httpProxyIndex = 0
	}

	oneBrowser, err := NewBrowserBase(b.rodOptions.BrowserFPath(), b.proxyInfos[b.httpProxyIndex].HttpUrl, b.rodOptions.LoadAdblock())
	if err != nil {
		return nil, errors.New("NewBrowser.NewBrowserBase error:" + err.Error())
	}

	return oneBrowser, nil
}

func (b *Browser) Close() {

	for _, oneBrowser := range b.multiBrowser {
		_ = oneBrowser.Close()
	}

	b.multiBrowser = make([]*rod.Browser, 0)
}

type ProxyResult struct {
	Status         string       `json:"status"`
	AppVersion     string       `json:"app_version"`
	LBPort         int          `json:"lb_port"`
	OpenResultList []OpenResult `json:"open_result_list"`
}

type OpenResult struct {
	Name       string `json:"name"`
	ProtoModel string `json:"proto_model"`
	SocksPort  int    `json:"socks_port"`
	HttpPort   int    `json:"http_port"`
}

type XrayPoolProxyInfo struct {
	Index           int
	Name            string  `json:"name"`
	ProtoModel      string  `json:"proto_model"`
	SocksUrl        string  `json:"socks_url"`
	HttpUrl         string  `json:"http_url"`
	skipAccessTime  int64   // 如果当前时间大于这个时间，这个节点才可以被访问
	lastAccessTime  int64   // 最后的访问时间
	accessTimeLines []int64 // 每一次访问时间的队列
}

const (
	httpPrefix  = "http://"
	socksPrefix = "socks5://"
)
