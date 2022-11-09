package rod_helper

import (
	"crypto/tls"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func NewBrowserBase(browserFPath, httpProxyURL string, loadAdblock, loadPic bool, preLoadUrl ...string) (*rod.Browser, error) {

	var err error
	// 随机的 rod 子文件夹名称
	nowUserData := filepath.Join(GetRodTmpRootFolder(), RandStringBytesMaskImprSrcSB(20))
	var browser *rod.Browser
	// 如果没有指定 chrome 的路径，则使用 rod 自行下载的 chrome
	err = rod.Try(func() {

		var nowLancher *launcher.Launcher
		purl := ""
		if loadAdblock == true {

			nowLancher = launcher.New().
				Delete("disable-extensions").
				Set("load-extension", GetADBlockLocalPath(httpProxyURL)).
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
		return nil, err
	}
	// 如果加载了插件，那么就需要进行一定地耗时操作，等待其第一次的加载完成
	if loadAdblock == true {

		if httpProxyURL == "" {
			page, _, _ := NewPageNavigate(browser, noProxyUseUrl, 15*time.Second)
			if page != nil {
				_ = page.Close()
			}
		} else {
			page, _, _ := NewPageNavigateWithProxy(browser, httpProxyURL, useProxyUrl, 15*time.Second)
			if page != nil {
				_ = page.Close()
			}
		}
	}

	if len(preLoadUrl) > 0 && preLoadUrl[0] != "" {

		if httpProxyURL == "" {
			page, _, _ := NewPageNavigate(browser, preLoadUrl[0], 15*time.Second)
			if page != nil {
				_ = page.Close()
			}
		} else {
			page, _, _ := NewPageNavigateWithProxy(browser, httpProxyURL, preLoadUrl[0], 15*time.Second)
			if page != nil {
				_ = page.Close()
			}
		}
	}

	return browser, nil
}

func NewPageNavigate(browser *rod.Browser, desURL string, timeOut time.Duration) (*rod.Page, *proto.NetworkResponseReceived, error) {

	page, err := newPage(browser)
	if err != nil {
		return nil, nil, err
	}

	return PageNavigate(page, desURL, timeOut)
}

func NewPageNavigateWithProxy(browser *rod.Browser, proxyUrl string, desURL string, timeOut time.Duration) (*rod.Page, *proto.NetworkResponseReceived, error) {

	page, err := newPage(browser)
	if err != nil {
		return nil, nil, err
	}

	return PageNavigateWithProxy(page, proxyUrl, desURL, timeOut)
}

func PageNavigate(page *rod.Page, desURL string, timeOut time.Duration) (*rod.Page, *proto.NetworkResponseReceived, error) {

	err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: RandomUserAgent(true),
	})
	if err != nil {
		if page != nil {
			_ = page.Close()
		}
		return nil, nil, err
	}
	var e proto.NetworkResponseReceived
	wait := page.WaitEvent(&e)
	err = rod.Try(func() {
		page.Timeout(timeOut).MustNavigate(desURL).MustWaitLoad()
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

func PageNavigateWithProxy(page *rod.Page, proxyUrl string, desURL string, timeOut time.Duration) (*rod.Page, *proto.NetworkResponseReceived, error) {

	router := page.HijackRequests()
	defer router.Stop()

	router.MustAdd("*", func(ctx *rod.Hijack) {
		px, _ := url.Parse(proxyUrl)
		nowTransport := &http.Transport{
			Proxy:           http.ProxyURL(px),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		defer func() {
			nowTransport.CloseIdleConnections()
			nowTransport = nil
		}()

		nowClient := &http.Client{
			Transport: nowTransport,
		}
		defer func() {
			nowClient.CloseIdleConnections()
			nowClient = nil
		}()

		err := ctx.LoadResponse(nowClient, false)
		if err != nil {
			return
		}
	})
	go router.Run()

	err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: RandomUserAgent(true),
	})
	if err != nil {
		if page != nil {
			page.Close()
		}
		return nil, nil, err
	}
	var e proto.NetworkResponseReceived
	wait := page.WaitEvent(&e)
	err = rod.Try(func() {
		page.Timeout(timeOut).MustNavigate(desURL).MustWaitLoad()
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

		publicIPPage, _, err := PageNavigate(page, publicIPSite, timeOut)
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

func SetNoProxyUseUrl(url string) {
	noProxyUseUrl = url
}

func SetUseProxyUrl(url string) {
	useProxyUrl = url
}

// ContainedWords 返回的页面是否包含关键词
func ContainedWords(pageContent string, failedWords []string) (bool, int) {

	for i, word := range failedWords {

		if strings.Contains(strings.ToLower(pageContent), word) == true {
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

func newPage(browser *rod.Browser) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: ""})
	if err != nil {
		return nil, err
	}
	return page, err
}

const regMatchIP = `(?m)((25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d))).){3}(25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d)))`

var ReMatchIP = regexp.MustCompile(regMatchIP)

var (
	noProxyUseUrl = "https://www.163.com"
	useProxyUrl   = "https://www.yahoo.com"
)
