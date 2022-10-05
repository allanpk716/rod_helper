package rod_helper

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	crx3 "github.com/mediabuyerbot/go-crx3"
	"github.com/pkg/errors"
	"path/filepath"
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
		time.Sleep(RandomSecondDuration(5, 10))
	}

	if len(preLoadUrl) > 0 && preLoadUrl[0] != "" {
		_, _, err = NewPageNavigate(browser, preLoadUrl[0], 15*time.Second)
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

func newPage(browser *rod.Browser) (*rod.Page, error) {
	page, err := browser.Page(proto.TargetCreateTarget{URL: ""})
	if err != nil {
		return nil, err
	}
	return page, err
}
