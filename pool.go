package rod_helper

import (
	_ "embed"
	"fmt"
	"github.com/WQGroup/logger"
	"github.com/go-resty/resty/v2"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-rod/rod"
)

type Pool struct {
	log             *logrus.Logger
	rodOptions      *PoolOptions         // 参数
	httpProxyIndex  int                  // 当前使用的 http 代理的索引
	httpProxyLocker sync.Mutex           // http 代理的锁
	lbHttpUrl       string               // 负载均衡的 http proxy url
	lbPort          int                  // 负载均衡 http 端口
	proxyInfos      []*XrayPoolProxyInfo // XrayPool 中的代理信息
}

// NewPool 面向与爬虫的时候使用 Pool
func NewPool(browserOptions *PoolOptions) *Pool {

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

	b := &Pool{
		log:        browserOptions.Log,
		rodOptions: browserOptions,
		proxyInfos: make([]*XrayPoolProxyInfo, 0),
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
	}
	b.lbPort = proxyResult.LBPort

	b.lbHttpUrl = fmt.Sprintf(httpPrefix + browserOptions.XrayPoolUrl() + ":" + strconv.Itoa(b.lbPort))

	return b
}

// GetOptions 获取设置的参数
func (b *Pool) GetOptions() *PoolOptions {
	return b.rodOptions
}

// LBPort 负载均衡 http 端口
func (b *Pool) LBPort() int {
	return b.lbPort
}

// LBHttpUrl 负载均衡的 http proxy url
func (b *Pool) LBHttpUrl() string {
	return b.lbHttpUrl
}

// GetOneProxyInfo 轮询获取一个代理实例，直接给出这个代理的信息，不会考虑访问的频率问题
func (b *Pool) GetOneProxyInfo() (*XrayPoolProxyInfo, error) {

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

	if b.httpProxyIndex > len(b.proxyInfos)-1 {
		b.httpProxyIndex = 0
	}
	if b.proxyInfos[b.httpProxyIndex].skipAccessTime > nowUnixTime {
		// 这个节点需要跳过
		return b.proxyInfos[b.httpProxyIndex], ErrSkipAccessTime
	}

	return b.proxyInfos[b.httpProxyIndex], nil
}

// SetProxyNodeSkipByTime 设置这个节点，等待多少秒之后才可以被再次使用，仅仅针对 GetOneProxyInfo、GetProxyInfoSync 有效
func (b *Pool) SetProxyNodeSkipByTime(index int, targetSkipTime int64) error {
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

	b.log.Infoln("SetProxyNodeSkipByTime", index, targetSkipTime)
	b.proxyInfos[index].skipAccessTime = targetSkipTime
	return nil
}

// GetProxyInfoSync 根据 TimeConfig 设置，寻找一个可用的节点。可以并发用，但是当前获取的节点，会根据访问时间，可能会有 sleep 阻塞等待
func (b *Pool) GetProxyInfoSync(baseUrl string) (*XrayPoolProxyInfo, error) {

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
func (b *Pool) PageStatusCodeCheck(e *proto.NetworkResponseReceived, statusCodeInfo []StatusCodeInfo, nowProxyInfo *XrayPoolProxyInfo, baseUrl string) (PageCheck, error) {

	doWhat := func(index int, codeInfo StatusCodeInfo) (PageCheck, error) {

		defer func() {
			logger.Warningln("PageStatusCodeCheck", codeInfo.Operator, codeInfo.Codes[index], codeInfo.WillDo, baseUrl)
		}()

		if codeInfo.NeedPunishment == true {
			// 需要进行惩罚
			err := b.SetProxyNodeSkipByTime(nowProxyInfo.Index, b.rodOptions.timeConfig.GetProxyNodeSkipAccessTime())
			if err != nil {
				return codeInfo.WillDo, err
			}
		}
		return codeInfo.WillDo, nil
	}

	if e != nil && e.Response != nil {

		for _, codeInfo := range statusCodeInfo {
			switch codeInfo.Operator {
			case Match:
				// 等于的情况
				for index, code := range codeInfo.Codes {
					if e.Response.Status == code {
						return doWhat(index, codeInfo)
					}
				}
			case GreatThan:
				// 大于的情况
				for index, code := range codeInfo.Codes {
					if e.Response.Status > code {
						return doWhat(index, codeInfo)
					}
				}
			case LessThan:
				// 小于的情况
				for index, code := range codeInfo.Codes {
					if e.Response.Status < code {
						return doWhat(index, codeInfo)
					}
				}
			default:
				// 其他情况跳过
				continue
			}
		}

		logger.Infoln("PageStatusCodeCheck", Success, baseUrl)
		// 都没踩中，那么就继续下面的逻辑吧
		return Success, nil
	} else {
		// 这个事件收不到，那么就是无法使用 page 获取 HTML 以及查询元素的操作的，会卡住一直等着，所以这里需要设置一下，跳过这个代理
		logger.Warningln("Response is nil", Repeat, nowProxyInfo.Name, baseUrl)
		return Repeat, nil
	}
}

// HasSuccessWord 是否包含成功的关键词，开启这个设置才有效，需要提前调用 PageStatusCodeCheck 判断，或者判断 proto.NetworkResponseReceived 的值
func (b *Pool) HasSuccessWord(page *rod.Page, nowProxyInfo *XrayPoolProxyInfo) (bool, error) {

	pageContent, err := page.HTML()
	if err != nil {
		return false, err
	}

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

	return true, nil
}

// HasFailedWord 是否包含失败关键词，开启这个设置才有效，需要提前调用 PageStatusCodeCheck 判断，或者判断 proto.NetworkResponseReceived 的值
func (b *Pool) HasFailedWord(page *rod.Page, nowProxyInfo *XrayPoolProxyInfo) (bool, string, error) {

	pageContent, err := page.HTML()
	if err != nil {
		return false, "", err
	}

	// 检查是否包含失败关键词
	contained, index := ContainedWords(pageContent, b.rodOptions.failWordsConfig.Words)
	if contained == true {
		// 如果包含了失败的关键词，那么就需要统计出来，到底最近访问这个节点的频率是如何的，提供给人来判断调整
		err = b.SetProxyNodeSkipByTime(nowProxyInfo.Index, b.rodOptions.timeConfig.GetProxyNodeSkipAccessTime())
		if err != nil {
			return false, "", err
		}
		// 找到了错误关键词
		return true, b.rodOptions.failWordsConfig.Words[index], nil
	}

	return false, "", nil
}

// NewBrowser 每次新建一个 Pool ，使用 HttpProxy 列表中的一个作为代理
func (b *Pool) NewBrowser() (*BrowserInfo, error) {

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

	cachePath := b.rodOptions.CacheRootDirPath()
	oneBrowserInfo, err := NewBrowserBase(cachePath,
		b.rodOptions.BrowserFPath(), b.proxyInfos[b.httpProxyIndex].HttpUrl,
		b.rodOptions.LoadAdblock(), b.rodOptions.LoadPicture())
	if err != nil {
		return nil, errors.New("NewBrowser.NewBrowserBase error:" + err.Error())
	}

	return oneBrowserInfo, nil
}

func (b *Pool) Close() {
	_ = os.RemoveAll(b.rodOptions.CacheRootDirPath())
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

type PageCheck int

const (
	Skip    PageCheck = iota + 1 // 跳过后续的逻辑，不需要再次访问
	Repeat                       // 跳过后续的逻辑，但是要求重新访问
	Success                      // 检查通过，继续后续的逻辑判断
)

func (s PageCheck) String() string {
	switch s {
	case Skip:
		return "Skip"
	case Repeat:
		return "Repeat"
	case Success:
		return "Success"
	default:
		return "Unknown"
	}
}
