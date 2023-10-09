package rod_helper

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/WQGroup/logger"
	"github.com/go-resty/resty/v2"
	"github.com/go-rod/rod/lib/proto"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"github.com/ysmood/gson"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-rod/rod"
)

type Pool struct {
	log               *logrus.Logger
	rodOptions        *PoolOptions                    // 参数
	httpProxyIndex    int                             // 当前使用的 http 代理的索引
	httpProxyLocker   sync.Mutex                      // http 代理的锁
	lbHttpUrl         string                          // 负载均衡的 http proxy url
	lbPort            int                             // 负载均衡 http 端口
	proxyInfos        []*XrayPoolProxyInfo            // XrayPool 中的代理信息
	filterProxyInfos  map[string][]*XrayPoolProxyInfo // 过滤后的代理信息
	filterProxyLocker sync.Mutex                      // 过滤代理的锁
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

	b.filterProxyInfos = make(map[string][]*XrayPoolProxyInfo)

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

// Filter 传入一批需要进行测试的 URL，然后过滤掉不可用的代理
func (b *Pool) Filter(fInfo *FilterInfo, threadSize int, tcpOrBrowserTest bool) error {

	if len(b.proxyInfos) < 1 {
		return ErrProxyInfosIsEmpty
	}
	statusCodeInfos := []StatusCodeInfo{
		{
			Codes:    []int{404},
			Operator: Match,
			WillDo:   Skip,
		},
		{
			Codes:    []int{499},
			Operator: GreatThan,
			WillDo:   Skip,
		},
		{
			Codes:    []int{403},
			Operator: Match,
			WillDo:   Repeat,
		},
	}

	var nowBrowser *BrowserInfo
	var err error
	if tcpOrBrowserTest == false {
		nowBrowser, err = b.NewBrowser()
		if err != nil {
			return err
		}
		defer nowBrowser.Close()
	}
	// 清理
	b.filterProxyInfos[fInfo.Key] = make([]*XrayPoolProxyInfo, 0)
	logger.Infoln("Pool.Filter", fInfo.Key, "Start...")
	var wg sync.WaitGroup
	p, err := ants.NewPoolWithFunc(threadSize, func(inData interface{}) {
		deliveryInfo := inData.(DeliveryInfo)
		defer func() {
			deliveryInfo.Wg.Done()
			logger.Infoln("Pool.Filter", deliveryInfo.ProxyInfo.Name, deliveryInfo.ProxyInfo.Index, "End")
		}()

		logger.Infoln("Pool.Filter", deliveryInfo.ProxyInfo.Name, deliveryInfo.ProxyInfo.Index, "Start...")

		// 测试这节点
		for _, pageInfo := range deliveryInfo.PageInfos {

			if deliveryInfo.tcpOrBrowserTest == true {
				speedResult, err := b.TryLoadUrl(deliveryInfo.ProxyInfo, pageInfo)
				if err != nil {
					logger.Errorf("Pool.Filter TryLoadUrl error: %v", err)
					continue
				}
				logger.Infoln("Pool.Filter", deliveryInfo.ProxyInfo.Name, deliveryInfo.ProxyInfo.Index, pageInfo.Name, speedResult)

			} else {

				speedResult, nowPage, err := b.TryLoadPage(deliveryInfo.Browser, deliveryInfo.ProxyInfo, pageInfo, statusCodeInfos, false)
				if err != nil {
					logger.Errorf("Pool.Filter TryLoadPage error: %v", err)
					continue
				}
				if nowPage != nil {
					_ = nowPage.Close()
				}
				logger.Infoln("Pool.Filter", deliveryInfo.ProxyInfo.Name, deliveryInfo.ProxyInfo.Index, pageInfo.Name, speedResult)
			}
			// 加入缓存列表
			b.filterProxyLocker.Lock()
			b.filterProxyInfos[fInfo.Key] = append(b.filterProxyInfos[fInfo.Key], deliveryInfo.ProxyInfo)
			b.filterProxyLocker.Unlock()
		}
	})
	if err != nil {
		return err
	}
	// 过滤
	for _, proxyInfo := range b.proxyInfos {
		wg.Add(1)

		err = p.Invoke(DeliveryInfo{
			Browser:          nowBrowser,
			ProxyInfo:        proxyInfo,
			PageInfos:        fInfo.PageInfos,
			Wg:               &wg,
			tcpOrBrowserTest: tcpOrBrowserTest,
		})
		if err != nil {
			logger.Errorf("Pool.Filter 工作池提交任务失败: %v", err)
		}
	}

	wg.Wait()

	logger.Infoln("Pool.Filter", fInfo.Key, "End")

	return nil
}

func (b *Pool) GetFilterProxyInfos(keyName string) ([]*XrayPoolProxyInfo, error) {
	if len(b.proxyInfos) < 1 {
		return nil, ErrProxyInfosIsEmpty
	}

	return b.filterProxyInfos[keyName], nil
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

// PageStatusCodeCheckBase 页面状态码检查
func (b *Pool) PageStatusCodeCheckBase(e *proto.NetworkResponseReceived, statusCodeInfo []StatusCodeInfo, baseUrl string) (PageCheck, error) {

	doWhat := func(index int, codeInfo StatusCodeInfo) (PageCheck, error) {

		defer func() {
			logger.Warningln("PageStatusCodeCheck", codeInfo.Operator, codeInfo.Codes[index], codeInfo.WillDo, baseUrl)
		}()

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
		logger.Warningln("Response is nil", Repeat, baseUrl)
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

// HasSuccessWordBase 是否包含成功的关键词，开启这个设置才有效，需要提前调用 PageStatusCodeCheck 判断，或者判断 proto.NetworkResponseReceived 的值
func (b *Pool) HasSuccessWordBase(page *rod.Page, words []string) (bool, error) {

	pageContent, err := page.HTML()
	if err != nil {
		return false, err
	}

	// 检查是否包含成功关键词
	contained, _ := ContainedWords(pageContent, words)
	if contained == false {
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

// HasFailedWordBase 是否包含失败关键词，开启这个设置才有效，需要提前调用 PageStatusCodeCheck 判断，或者判断 proto.NetworkResponseReceived 的值
func (b *Pool) HasFailedWordBase(page *rod.Page, words []string) (bool, string, error) {

	pageContent, err := page.HTML()
	if err != nil {
		return false, "", err
	}

	// 检查是否包含失败关键词
	contained, index := ContainedWords(pageContent, words)
	if contained == true {
		// 找到了错误关键词
		return true, words[index], nil
	}

	return false, "", nil
}

// NewBrowser 每次新建一个 Browser ，不使用代理
func (b *Pool) NewBrowser() (*BrowserInfo, error) {

	cachePath := b.rodOptions.CacheRootDirPath()
	oneBrowserInfo, err := NewBrowserBase(cachePath,
		b.rodOptions.BrowserFPath(), "",
		b.rodOptions.LoadAdblock(), b.rodOptions.LoadPicture())
	if err != nil {
		return nil, errors.New("NewBrowser.NewBrowserBase error:" + err.Error())
	}

	return oneBrowserInfo, nil
}

// NewBrowserWithRandomProxy 每次新建一个 Browser ，使用 HttpProxy 列表中的一个作为代理
func (b *Pool) NewBrowserWithRandomProxy() (*BrowserInfo, error) {

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
		return nil, errors.New("NewBrowserWithRandomProxy.NewBrowserBase error:" + err.Error())
	}

	return oneBrowserInfo, nil
}

// TryLoadPage 只关心以现在有的信息去尝试加载一个页面，不考虑其中可能遇到验证码的情况
func (b *Pool) TryLoadPage(browserInfo *BrowserInfo, nowProxyInfo *XrayPoolProxyInfo,
	pageInfo PageInfo, statusCodeInfos []StatusCodeInfo, needRedirect bool) (int, *rod.Page, error) {

	var err error
	var page *rod.Page
	var client *resty.Client
	var e *proto.NetworkResponseReceived

	timeOut := pageInfo.GetPageTimeOut()
	if needRedirect == true {
		timeOut = pageInfo.GetPageTimeOut() + time.Second*45
	}
	// --------------------------------- 1. 加载页面 ---------------------------------
	// 新建一个 page 使用
	logger.Infoln("NowProxy:", nowProxyInfo.Name)
	opt := NewHttpClientOptions(timeOut)
	opt.SetHttpProxy(nowProxyInfo.HttpUrl)
	client, err = NewHttpClient(opt)
	if err != nil {
		return -1, nil, err
	}
	start := time.Now()
	page, err = NewPage(browserInfo.Browser)
	if err != nil {
		return -1, nil, err
	}
	defer func() {
		if err != nil && page != nil {
			logger.Infoln("TryLoadPage -> Try Close Page")
			_ = page.Close()
		}
	}()
	err = page.SetWindow(&proto.BrowserBounds{
		Left:        gson.Int(0),
		Top:         gson.Int(50),
		Width:       gson.Int(900),
		Height:      gson.Int(900),
		WindowState: proto.BrowserWindowStateNormal,
	})
	router := NewPageHijackRouter(page, true, client.GetClient())
	defer func() {
		_ = router.Stop()
	}()
	go router.Run()
	// 设置代理
	page, e, err = PageNavigate(
		page, true, pageInfo.Url,
		timeOut,
	)
	if err != nil {
		// 这里可能会出现超时，但是实际上是成功的，所以这里不需要返回错误
		if errors.Is(err, context.DeadlineExceeded) == false {
			// 不是超时错误，那么就返回错误，跳过
			return -1, nil, err
		}
	}
	err = page.Timeout(timeOut).WaitLoad()
	if err != nil {
		// 这里可能会出现超时，但是实际上是成功的，所以这里不需要返回错误
		if errors.Is(err, context.DeadlineExceeded) == false {
			// 不是超时错误，那么就返回错误，跳过
			return -1, nil, err
		}
	}
	// ------------------判断返回值是否符合期望------------------
	logger.Infoln(pageInfo.Name, "PageStatusCodeCheck: ", pageInfo.Url)
	var StatusCodeCheck PageCheck
	StatusCodeCheck, err = b.PageStatusCodeCheckBase(e, statusCodeInfos, pageInfo.Url)
	if err != nil {
		return -1, nil, err
	}
	switch StatusCodeCheck {
	case Skip:
		// 跳过后续的逻辑，不需要再次访问
		logger.Warningln("PageStatusCodeCheck Skip NeedSkipProxyIndexList", nowProxyInfo.Index, nowProxyInfo.Name)
		err = errors.New(pageInfo.Name + " PageStatusCodeCheck Skip")
		return -1, nil, err
	case Repeat:
		logger.Warningln("PageStatusCodeCheck Repeat NeedSkipProxyIndexList", nowProxyInfo.Index, nowProxyInfo.Name)
		// 重新访问，需要再次请求这个页面
		err = errors.New(pageInfo.Name + " PageStatusCodeCheck Repeat")
		return -1, nil, err
	}
	// 激活界面
	_, err = page.Activate()
	if err != nil {
		err = errors.New(pageInfo.Name + " Activate Error: " + err.Error())
		return -1, nil, err
	}
	// ------------------会循环检测是否加载完毕，关键 Ele 出现即可------------------
	logger.Infoln(pageInfo.Name, "HasPageLoaded: ", pageInfo.Url)
	pageLoaded := HasPageLoaded(page, pageInfo.ExistElementXPaths, pageInfo.PageTimeOut)
	logger.Infoln(pageInfo.Name, "HasPageLoaded: ", pageInfo.Url, pageLoaded)
	// 要在 StatusCode 检查之后再判断
	if pageLoaded == false {
		err = ErrPageLoadFailed
		return -1, nil, err
	}
	// ------------------是否包含成功关键词------------------
	if pageInfo.HasSuccessWord() == true {
		var bok bool
		logger.Infoln("HasSuccessWords: ", pageInfo.Url)
		bok, err = b.HasSuccessWordBase(page, pageInfo.SuccessWord)
		logger.Infoln("HasSuccessWords: ", pageInfo.Url, bok)
		if err != nil {
			err = errors.New(fmt.Sprintf("hasSuccessWord error: %s", err.Error()))
			return -1, nil, err
		}
		if bok == false {
			// 需要再次请求这个页面
			err = errors.New(pageInfo.Name + " Not Contained SuccessWord")
			return -1, nil, err
		}
	}
	elapsed := time.Since(start)
	speedResult := int(float32(elapsed.Nanoseconds()) / 1e6)

	return speedResult, page, nil
}

// TryLoadUrl 实现一个 http client 访问 url 的功能
func (b *Pool) TryLoadUrl(nowProxyInfo *XrayPoolProxyInfo, pageInfo PageInfo) (int, error) {

	logger.Infoln("NowProxy:", nowProxyInfo.Name)
	opt := NewHttpClientOptions(pageInfo.GetPageTimeOut())
	opt.SetHttpProxy(nowProxyInfo.HttpUrl)
	client, err := NewHttpClient(opt)
	if err != nil {
		return -1, err
	}

	start := time.Now()
	res, err := client.R().Get(pageInfo.Url)
	elapsed := time.Since(start)
	if err != nil {
		return -1, err
	}

	speedResult := int(float32(elapsed.Nanoseconds()) / 1e6)
	if res.StatusCode() != http.StatusOK {
		return -1, errors.New("StatusCode is not 200")
	}

	pageHtmlString := string(res.Body())
	if pageInfo.HasSuccessWord() == true {
		logger.Infoln("HasSuccessWords: ", pageInfo.Url)
		contained, _ := ContainedWords(pageHtmlString, pageInfo.SuccessWord)
		logger.Infoln("HasSuccessWords: ", pageInfo.Url, contained)
		if contained == false {
			// 需要再次请求这个页面
			err = errors.New(pageInfo.Name + " Not Contained SuccessWord")
			return -1, err
		}
	}

	return speedResult, nil
}

func (b *Pool) Close() {

	time.AfterFunc(time.Second*5, func() {
		_ = os.RemoveAll(b.rodOptions.CacheRootDirPath())
	})
}
