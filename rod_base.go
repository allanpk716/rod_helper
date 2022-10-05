package rod_helper

import (
	"crypto/tls"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	crx3 "github.com/mediabuyerbot/go-crx3"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func NewBrowserBase(httpProxyURL string, loadAdblock bool, preLoadUrl ...string) (*rod.Browser, error) {

	var err error
	// 随机的 rod 子文件夹名称
	nowUserData := filepath.Join(GetRodTmpRootFolder(), RandStringBytesMaskImprSrcSB(20))
	var browser *rod.Browser
	// 如果没有指定 chrome 的路径，则使用 rod 自行下载的 chrome
	err = rod.Try(func() {
		purl := ""
		if loadAdblock == true {

			desFile, err := GetADBlock(httpProxyURL)
			if err != nil {
				panic(errors.New(fmt.Sprintf("get adblock failed: %s", err)))
			}
			if err = crx3.Extension(desFile).Unpack(); err != nil {
				panic(errors.New("unpack adblock failed: " + err.Error()))
			}
			filenameOnly := strings.TrimSuffix(filepath.Base(desFile), filepath.Ext(desFile))

			purl = launcher.New().
				Delete("disable-extensions").
				Set("load-extension", filepath.Join(GetADBlockFolder(), filenameOnly)).
				Proxy(httpProxyURL).
				Headless(false). // 插件模式需要设置这个
				UserDataDir(nowUserData).
				//XVFB("--server-num=5", "--server-args=-screen 0 1600x900x16").
				//XVFB("-ac :99", "-screen 0 1280x1024x16").
				MustLaunch()
		} else {
			purl = launcher.New().
				Proxy(httpProxyURL).
				UserDataDir(nowUserData).
				MustLaunch()
		}

		browser = rod.New().ControlURL(purl).MustConnect()
	})
	if err != nil {
		return nil, err
	}
	// 如果加载了插件，那么就需要进行一定地耗时操作，等待其第一次的加载完成
	if loadAdblock == true {

		const mainlandUrl = "https://www.163.com"
		const outsideUrl = "https://www.yahoo.com"
		strTageSite := ""
		if httpProxyURL == "" {
			strTageSite = mainlandUrl
		} else {
			strTageSite = outsideUrl
		}
		page, _, err := NewPageNavigate(browser, strTageSite, 15*time.Second)
		if err != nil {
			if browser != nil {
				_ = browser.Close()
			}
			return nil, err
		}
		err = page.WaitLoad()
		if err != nil {
			if browser != nil {
				_ = browser.Close()
			}
			return nil, err
		}
		//time.Sleep(RandomSecondDuration(5, 10))
	}

	if len(preLoadUrl) > 0 && preLoadUrl[0] != "" {
		page, _, err := NewPageNavigate(browser, preLoadUrl[0], 15*time.Second)
		if err != nil {
			if browser != nil {
				_ = browser.Close()
			}
			return nil, err
		}
		err = page.WaitLoad()
		if err != nil {
			if browser != nil {
				_ = browser.Close()
			}
			return nil, err
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
	page = page.Timeout(timeOut)
	err = rod.Try(func() {
		page.MustNavigate(desURL)
		wait()
	})
	if err != nil {
		if page != nil {
			_ = page.Close()
		}
		return nil, nil, err
	}
	// 出去前把 TimeOUt 取消了
	page = page.CancelTimeout()

	return page, &e, nil
}

func PageNavigateWithProxy(page *rod.Page, proxyUrl string, desURL string, timeOut time.Duration) (*rod.Page, *proto.NetworkResponseReceived, error) {

	router := page.HijackRequests()
	defer router.Stop()

	router.MustAdd("*", func(ctx *rod.Hijack) {
		px, _ := url.Parse(proxyUrl)
		err := ctx.LoadResponse(&http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyURL(px),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}, true)
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
	page = page.Timeout(timeOut)
	err = rod.Try(func() {
		page.MustNavigate(desURL)
		wait()
	})
	if err != nil {
		if page != nil {
			page.Close()
		}
		return nil, nil, err
	}
	// 出去前把 TimeOUt 取消了
	page = page.CancelTimeout()
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

func newPage(browser *rod.Browser) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: ""})
	if err != nil {
		return nil, err
	}
	return page, err
}

const regMatchIP = `(?m)((25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d))).){3}(25[0-5]|2[0-4]\d|((1\d{2})|([1-9]?\d)))`

var ReMatchIP = regexp.MustCompile(regMatchIP)
