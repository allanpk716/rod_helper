package rod_helper

import (
	"github.com/WQGroup/logger"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func NewBrowserBase(tmpRootFolder, browserFPath, httpProxyURL string, loadAdblock, loadPic bool) (*BrowserInfo, error) {

	var err error
	// 随机的 rod 子文件夹名称
	nowUserData := filepath.Join(GetRodTmpRootFolder(tmpRootFolder), RandStringBytesMaskImprSrcSB(20))
	err = os.MkdirAll(nowUserData, os.ModePerm)
	if err != nil {
		return nil, err
	}
	var browser *rod.Browser
	// 如果没有指定 chrome 的路径，则使用 rod 自行下载的 chrome
	err = rod.Try(func() {

		var nowLancher *launcher.Launcher
		purl := ""
		if loadAdblock == true {

			nowLancher = launcher.New().
				Delete("disable-extensions").
				// 这里要写的是缓存的根目录，不是 Browser 的目录
				Set("load-extension", GetADBlockLocalPath(tmpRootFolder, httpProxyURL)).
				Proxy(httpProxyURL).
				Headless(false). // 插件模式需要设置这个
				UserDataDir(nowUserData)
			//XVFB("--server-num=5", "--server-args=-screen 0 1600x900x16").
			//XVFB("-ac :99", "-screen 0 1280x1024x16").
		} else {
			nowLancher = launcher.New().
				Proxy(httpProxyURL).
				UserDataDir(nowUserData)
		}

		if loadPic == false {
			nowLancher.Set("blink-settings", "imagesEnabled=false")
		}

		if browserFPath != "" {
			// 指定浏览器启动
			nowLancher = nowLancher.Bin(browserFPath)
		}
		purl = nowLancher.MustLaunch()
		browser = rod.New().ControlURL(purl).MustConnect()
	})
	if err != nil {
		_ = os.RemoveAll(nowUserData)
		return nil, err
	}

	// 忽略证书错误
	err = browser.IgnoreCertErrors(true)
	if err != nil {
		return nil, err
	}

	return NewBrowserInfo(browser, nowUserData), nil
}

func NewPage(browser *rod.Browser) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: ""})
	if err != nil {
		return nil, err
	}
	return page, err
}

func PageNavigate(page *rod.Page, randomUA bool, desURL string, timeOut time.Duration) (*rod.Page, *proto.NetworkResponseReceived, error) {

	if randomUA == true {
		ua := RandomUserAgent()
		err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: ua,
		})
		if err != nil {
			if page != nil {
				_ = page.Close()
			}
			return nil, nil, err
		}
	}
	var e proto.NetworkResponseReceived
	wait := page.WaitEvent(&e)
	err := rod.Try(func() {
		page.Timeout(timeOut).MustNavigate(desURL)
		wait()
	})
	if err != nil {
		return page, &e, err
	}
	if page == nil {
		return nil, nil, errors.New("page is nil")
	}

	return page, &e, nil
}

// NewPageHijackRouter 需要手动启动 休要开启协程 Run() 和 释放 Stop
func NewPageHijackRouter(page *rod.Page, loadBody bool, httpClient *http.Client) *rod.HijackRouter {
	router := page.HijackRequests()
	router.MustAdd("*", func(ctx *rod.Hijack) {
		err := ctx.LoadResponse(httpClient, loadBody)
		if err != nil {
			return
		}
	})
	return router
}

// ContainedWords 返回的页面是否包含关键词
func ContainedWords(pageContent string, failedWords []string) (bool, int) {

	for i, word := range failedWords {

		if strings.Contains(strings.ToLower(pageContent), strings.ToLower(word)) == true {
			return true, i
		}
	}
	return false, -1
}

// ContainedWordsRegex 返回的页面是否包含关键词正则表达式
func ContainedWordsRegex(pageContent string, failedWordsRegex []string) (bool, int) {

	for i, wordRegex := range failedWordsRegex {

		failedRegex := regexp.MustCompile(wordRegex)
		matches := failedRegex.FindAllString(pageContent, -1)
		if matches == nil || len(matches) == 0 {
			// 没有找到匹配的内容，那么认为是成功的
		} else {
			return true, i
		}
	}

	return false, -1
}

// PageStatusCodeCheck 页面状态码检查
func PageStatusCodeCheck(e *proto.NetworkResponseReceived, statusCodeInfo []StatusCodeInfo) (PageCheck, error) {

	doWhat := func(index int, codeInfo StatusCodeInfo) (PageCheck, error) {

		defer func() {
			logger.Warningln("PageStatusCodeCheck", codeInfo.Operator, codeInfo.Codes[index], codeInfo.WillDo)
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

		logger.Infoln("PageStatusCodeCheck", Success)
		// 都没踩中，那么就继续下面的逻辑吧
		return Success, nil
	} else {
		// 这个事件收不到，那么就是无法使用 page 获取 HTML 以及查询元素的操作的，会卡住一直等着，所以这里需要设置一下，跳过这个代理
		logger.Warningln("Skip, Response is nil")
		return Repeat, nil
	}
}

func GetPublicIP(page *rod.Page, timeOut time.Duration, customDectIPSites []string) (string, error) {
	defPublicIPSites := []string{
		"https://myip.biturl.top/",
		"https://ip4.seeip.org/",
		"https://ipecho.net/plain",
		"https://api-ipv4.ip.sb/ip",
		"https://api.ipify.org/",
		"http://myexternalip.com/raw",
	}

	customPublicIPSites := make([]string, 0)
	if customDectIPSites != nil {
		customPublicIPSites = append(customPublicIPSites, customDectIPSites...)
	} else {
		customPublicIPSites = append(customPublicIPSites, defPublicIPSites...)
	}

	for _, publicIPSite := range customPublicIPSites {

		publicIPPage, _, err := PageNavigate(page, true, publicIPSite, timeOut)
		if err != nil {
			return "", err
		}
		html, err := publicIPPage.HTML()
		if err != nil {
			return "", err
		}
		matcheds := ReMatchIP.FindAllString(html, -1)
		if html != "" && matcheds != nil && len(matcheds) >= 1 {
			return matcheds[0], nil
		}
	}

	return "", errors.New("get public ip failed")
}

// HasPageLoaded 通过一个 Element 的 XPath 判断是否页面加载完毕
func HasPageLoaded(page *rod.Page, targetElementXPaths []string, timeOut int) bool {

	for {
		for _, targetElementXPath := range targetElementXPaths {
			foundEle, _, _ := page.Timeout(300 * time.Millisecond).HasX(targetElementXPath)
			if foundEle == true {
				return true
			}
		}
		// Timeout check
		if timeOut < 1 {
			break
		}
		timeOut--
		time.Sleep(1 * time.Second)
	}
	return false
}

const regMatchIP = `(?m)((25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d))).){3}(25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d)))`

var ReMatchIP = regexp.MustCompile(regMatchIP)

type BrowserInfo struct {
	Browser     *rod.Browser // 浏览器
	UserDataDir string       // 这里实例的缓存文件夹
}

func NewBrowserInfo(browser *rod.Browser, userDataDir string) *BrowserInfo {
	return &BrowserInfo{Browser: browser, UserDataDir: userDataDir}
}

func (bi *BrowserInfo) Close() {

	needClearFolder := bi.UserDataDir

	if bi.Browser != nil {
		_ = bi.Browser.Close()
		bi.Browser = nil
	}
	if needClearFolder != "" {

		if IsDir(needClearFolder) == false {
			return
		}
		logger.Infoln("try clear UserDataDir:", needClearFolder)

		time.AfterFunc(5*time.Second, func() {
			err := os.RemoveAll(needClearFolder)
			if err != nil {
				logger.Errorln("clear UserDataDir failed:", err)
			} else {
				logger.Infoln("clear UserDataDir success")
			}
		})
	} else {
		logger.Warningln("UserDataDir is empty")
	}
}
