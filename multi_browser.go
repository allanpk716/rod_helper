package rod_helper

import (
	_ "embed"
	"fmt"
	"github.com/WQGroup/logger"
	"github.com/go-resty/resty/v2"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-rod/rod"
)

type Browser struct {
	log             *logrus.Logger
	rodOptions      *BrowserOptions      // 参数
	multiBrowser    []*rod.Browser       // 多浏览器实例
	browserIndex    int                  // 当前使用的浏览器的索引
	browserLocker   sync.Mutex           // 浏览器的锁
	httpProxyIndex  int                  // 当前使用的 http 代理的索引
	httpProxyLocker sync.Mutex           // http 代理的锁
	LbHttpUrl       string               // 负载均衡的 http proxy url
	LBPort          int                  //负载均衡 http 端口
	proxyInfos      []*XrayPoolProxyInfo // XrayPool 中的代理信息
	httpClient      []*resty.Client      // http 客户端，有多少个代理，就是有多少个客户端
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
		proxyInfos:   make([]*XrayPoolProxyInfo, 0),
	}

	for index, result := range proxyResult.OpenResultList {

		// 单个节点的信息
		tmpProxyInfos := XrayPoolProxyInfo{
			Index:          index,
			Name:           result.Name,
			ProtoModel:     result.ProtoModel,
			HttpUrl:        httpPrefix + browserOptions.XrayPoolUrl() + ":" + strconv.Itoa(result.HttpPort),
			SocksUrl:       socksPrefix + browserOptions.XrayPoolUrl() + ":" + strconv.Itoa(result.SocksPort),
			FirTimeAccess:  true,
			skipAccessTime: 0,
			lastAccessTime: 0,
		}
		b.proxyInfos = append(b.proxyInfos, &tmpProxyInfos)
		// 对应一个 http client
		b.httpClient = append(b.httpClient, NewHttpClient(tmpProxyInfos.HttpUrl, 15*time.Second))
	}
	b.LBPort = proxyResult.LBPort

	b.LbHttpUrl = fmt.Sprintf(httpPrefix + browserOptions.XrayPoolUrl() + ":" + strconv.Itoa(b.LBPort))
	for i := 0; i < browserOptions.BrowserInstanceCount(); i++ {

		oneBrowser, err := NewBrowserBase(browserOptions.BrowserFPath(), b.LbHttpUrl,
			browserOptions.LoadAdblock(), browserOptions.LoadPicture())
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

// GetOneProxyInfo 轮询获取一个代理实例，直接给出这个代理的信息，不会考虑访问的频率问题
func (b *Browser) GetOneProxyInfo() (*XrayPoolProxyInfo, error) {

	b.httpProxyLocker.Lock()
	nowUnixTime := time.Now().Unix()
	defer func() {
		// 记录最后一次获取这个 Index ProxyInfo 的 UnixTime
		b.proxyInfos[b.httpProxyIndex].lastAccessTime = nowUnixTime
		// 下一个节点
		b.httpProxyIndex++
		if b.httpProxyIndex >= len(b.proxyInfos) {
			b.httpProxyIndex = 0
		}
		b.httpProxyLocker.Unlock()
	}()

	if len(b.proxyInfos) < 1 {
		return nil, ErrProxyInfosIsEmpty
	}

	if b.proxyInfos[b.httpProxyIndex].skipAccessTime > nowUnixTime {
		// 这个节点需要跳过
		return b.proxyInfos[b.httpProxyIndex], ErrSkipAccessTime
	}

	return b.proxyInfos[b.httpProxyIndex], nil
}

// GetHttpClient 配合 GetOneProxyInfo 使用，获取到一个 HttpClient
func (b *Browser) GetHttpClient(index int) *resty.Client {
	return b.httpClient[index]
}

// SetProxyNodeSkipByTime 设置这个节点，等待多少秒之后才可以被再次使用，仅仅针对 GetOneProxyInfo、GetProxyInfoSync 有效
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

// GetProxyInfoSync 根据 TimeConfig 设置，寻找一个可用的节点。可以并发用，但是当前获取的节点，会根据访问时间，可能会有 sleep 阻塞等待
func (b *Browser) GetProxyInfoSync(baseUrl string) (*XrayPoolProxyInfo, error) {

	var outProxyInfo *XrayPoolProxyInfo
	var err error
	for {
		// 获取下一个代理的信息
		outProxyInfo, err = b.GetOneProxyInfo()
		if err != nil {
			// 这里的错误要区分一种，就是跳过的节点的情况
			if errors.Is(err, ErrSkipAccessTime) {
				// 可以接收的错误，等待循环
				b.log.Debugln("Skip Access Time Proxy:", outProxyInfo.Name, outProxyInfo.Index, baseUrl)
				<-time.After(time.Microsecond * 100)
				continue
			}
			return nil, errors.Errorf("browser.GetOneProxyInfo error: %s", err.Error())
		}

		if outProxyInfo.FirTimeAccess == true {
			// 第一次访问，不需要等待
			outProxyInfo.FirTimeAccess = false
			return outProxyInfo, nil
		}

		timeT := time.Unix(outProxyInfo.GetLastAccessTime(), 0)
		dv := time.Now().Unix() - outProxyInfo.GetLastAccessTime()
		b.log.Infoln("Now Proxy:", outProxyInfo.Name, outProxyInfo.Index, timeT.Format("2006-01-02 15:04:05"), baseUrl)
		if dv > 0 && dv <= int64(b.rodOptions.timeConfig.OneProxyNodeUseInternalMinTime) {
			// 如果没有超过，那么就等待一段时间，然后再去获取
			// 休眠一下
			sleepTime := b.rodOptions.timeConfig.GetOneProxyNodeUseInternalTime(int32(dv))
			b.log.Infoln("Will Sleep", sleepTime.Seconds(), "s")
			<-time.After(sleepTime)
		} else if dv < 0 {
			// 理论上就不该到这个分支
			b.log.Warningln(outProxyInfo.Name, outProxyInfo.Index, "LastAccessTime is bigger than now time")
		}

		return outProxyInfo, nil
	}
}

// PageStatusCodeCheck 页面状态码检查
func (b *Browser) PageStatusCodeCheck(e *proto.NetworkResponseReceived, nowProxyInfo *XrayPoolProxyInfo, baseUrl string) (StatusCodeCheck, error) {

	if e != nil && e.Response != nil {

		if e.Response.Status == 404 || e.Response.Status >= 500 {
			// 这个页面有问题，跳过后续的逻辑，不再使用其他代理继续处理这个页面
			logger.Warningln("Skip, Status Code:", e.Response.Status, baseUrl)
			return Skip, nil
		} else if e.Response.Status == 403 {
			// 403，可能是被封了，需要换代理，设置时间惩罚，然后跳过
			err := errors.Errorf("403, Status Code: %d %s", e.Response.Status, baseUrl)
			logger.Warningln(err)
			err = b.SetProxyNodeSkipByTime(nowProxyInfo.Index, b.rodOptions.timeConfig.GetProxyNodeSkipAccessTime())
			if err != nil {
				return Repeat, err
			}
			return Repeat, err
		}
	}
	return Success, nil
}

// HasSuccessWord 是否包含成功的关键词，开启这个设置才有效
func (b *Browser) HasSuccessWord(page *rod.Page, nowProxyInfo *XrayPoolProxyInfo) (bool, error) {

	pageContent, err := page.HTML()
	if err != nil {
		return false, err
	}
	if b.rodOptions.successWordsConfig.Enable == true {
		// 检查是否包含成功关键词
		contained, _ := ContainedWords(pageContent, b.rodOptions.successWordsConfig.Words)
		if contained == false {

			// 如果没有包含成功的关键词，那么给予惩罚时间，这样就会暂时跳过这个代理节点
			err = b.SetProxyNodeSkipByTime(nowProxyInfo.Index, b.rodOptions.timeConfig.GetProxyNodeSkipAccessTime())
			if err != nil {
				return false, err
			}

			return false, nil
		}
	}

	return true, nil
}

// HasFailedWord 是否包含失败关键词
func (b *Browser) HasFailedWord(page *rod.Page, nowProxyInfo *XrayPoolProxyInfo) (bool, string, error) {
	pageContent, err := page.HTML()
	if err != nil {
		return false, "", err
	}
	if b.rodOptions.failWordsConfig.Enable == true {
		// 检查是否包含失败关键词
		contained, index := ContainedWords(pageContent, b.rodOptions.failWordsConfig.Words)
		if contained == true {
			// 如果包含了失败的关键词，那么就需要统计出来，到底最近访问这个节点的频率是如何的，提供给人来判断调整
			err = b.SetProxyNodeSkipByTime(nowProxyInfo.Index, b.rodOptions.timeConfig.GetProxyNodeSkipAccessTime())
			if err != nil {
				return false, "", err
			}
			return true, b.rodOptions.failWordsConfig.Words[index], nil
		}
	}

	return false, "", nil
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

	oneBrowser, err := NewBrowserBase(b.rodOptions.BrowserFPath(), b.proxyInfos[b.httpProxyIndex].HttpUrl,
		b.rodOptions.LoadAdblock(), b.rodOptions.LoadPicture())
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
	Index          int
	Name           string `json:"name"`
	ProtoModel     string `json:"proto_model"`
	SocksUrl       string `json:"socks_url"`
	HttpUrl        string `json:"http_url"`
	FirTimeAccess  bool   `json:"first_time_access"` // 这个节点第一次被访问
	skipAccessTime int64  // 如果当前时间大于这个时间，这个节点才可以被访问
	lastAccessTime int64  // 最后的访问时间
}

func (x *XrayPoolProxyInfo) GetLastAccessTime() int64 {
	return x.lastAccessTime
}

const (
	httpPrefix  = "http://"
	socksPrefix = "socks5://"
)

type StatusCodeCheck int

const (
	Skip    StatusCodeCheck = iota + 1 // 跳过后续的逻辑，不需要再次访问
	Repeat                             // 跳过后续的逻辑，但是要求重新访问
	Success                            // 检查通过，继续后续的逻辑判断
)
