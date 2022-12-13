package rod_helper

import (
	_ "embed"
	"github.com/WQGroup/logger"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

func InitFakeUA(tmpRootFolder, httpProxyURL string) {

	// 初始化
	browserUAs = [][]byte{ChromeJson, EdgeJson, FirefoxJson, OperaJson, SafariJson, MozillaJson}

	var err error
	// 查看本地是否有缓存
	uaRootPath := filepath.Join(".", "cache", "ua")
	if IsDir(uaRootPath) == false {
		err = GetFakeUserAgentDataCache(tmpRootFolder, httpProxyURL)
		if err != nil {
			logger.Warningln("GetFakeUserAgentDataCache Error, will load inside cache:", err)
			// 如果读取失败，就从本程序中获取缓存好的信息来使用
			readLocalCache(tmpRootFolder, httpProxyURL, false)
		}
	} else {
		// 从本地获取
		readLocalCache(tmpRootFolder, httpProxyURL, true)
	}
	logger.Infoln("InitFakeUA Done:", len(allUANames))
}

func readLocalCache(tmpRootFolder, httpProxyURL string, outSideAssets bool)  {

	var err error

	if outSideAssets == true {
		// 从外部获取
		uaRootPath := filepath.Join(".", "cache", "ua")
		for i, subType := range subTypes {

			uaFilePath := filepath.Join(uaRootPath, subType+".json")
			if IsFile(uaFilePath) == false {
				err = GetFakeUserAgentDataCache(tmpRootFolder, httpProxyURL)
				if err != nil {
					logger.Panicln(err)
				}
			}
			uaInfo := UserAgentInfo{}
			err = ToStruct(uaFilePath, &uaInfo)
			if err != nil {
				logger.Panicln(err)
			}
			allUANames = append(allUANames, uaInfo.UserAgents...)
			logger.Infoln(i, subType, len(uaInfo.UserAgents))
		}
	} else {
		for i, browserUA := range browserUAs {

			uaInfo := UserAgentInfo{}
			err = BytesToStruct(browserUA, &uaInfo)
			if err != nil {
				logger.Panicln(err)
			}
			allUANames = append(allUANames, uaInfo.UserAgents...)
			logger.Infoln(i, uaInfo.SubType, len(uaInfo.UserAgents))
		}
	}
}

func GetFakeUserAgentDataCache(tmpRootFolder, httpProxyURL string) error {

	/*
		暂时只获取：
		1. Browsers
		子分类中，再细化一点，只获取：
		// 桌面浏览器
		1. Chrome
		2. Edge
		3. Firefox
		4. Opera
		5. Safari
		6. Mozilla
	*/
	// 直接查找所有的 A 的链接，然后读取 href 信息，匹配 <a href="/pages/Chrome/ " class="unterMenuName">Chrome</a>
	nowBrowser, err := NewBrowserBase(tmpRootFolder, "", httpProxyURL, false, false)
	if err != nil {
		return err
	}
	defer nowBrowser.Close()
	var nowPage *rod.Page
	nowPage, err = NewPage(nowBrowser.Browser)
	if err != nil {
		return err
	}
	defer func() {
		_ = nowPage.Close()
	}()

	err = parseUAAllPage(nowPage)
	if err != nil {
		return err
	}

	return nil
}

func parseUAAllPage(nowPage *rod.Page) error {

	// 所有的 UA 的 SubType 都在这里
	const allInfoPage = "https://useragentstring.com/pages/useragentstring.php"
	var err error
	var p *proto.NetworkResponseReceived
	nowPage, p, err = PageNavigate(nowPage, false, allInfoPage, 15*time.Second)
	if err != nil {
		return err
	}
	statusCode := StatusCodeInfo{
		Codes:          []int{403},
		Operator:       Match,
		WillDo:         Skip,
		NeedPunishment: false,
	}
	StatusCodeCheck, err := PageStatusCodeCheck(
		p,
		[]StatusCodeInfo{statusCode})
	if err != nil {
		return err
	}
	switch StatusCodeCheck {
	case Skip, Repeat:
		// 跳过后续的逻辑，不需要再次访问
		return errors.New("StatusCodeCheck Error")
	}
	pageAllXPath := "//*[@id=\"menu\"]/a[2]"
	pageLoaded := HasPageLoaded(nowPage, pageAllXPath, 15)
	if pageLoaded == false {
		return errors.New("HasPageLoaded == false")
	}

	uaUrlMap := make(map[string][]string, 0)
	uaResultMap := make(map[string][]string, 0)

	err = rod.Try(func() {
		// 遍历所有的 A 的链接，然后读取 href 信息，匹配 <a href="/pages/Chrome/ " class="unterMenuName">Chrome</a>
		aEls := nowPage.MustElements("a")
		for i, el := range aEls {

			elString := el.MustText()
			if isSupportUAName(elString) == false {
				continue
			}
			nowElUrlPath := el.MustProperty("href")
			logger.Infoln(i, elString, nowElUrlPath)
			_, found := uaUrlMap[elString]
			if found == false {
				uaUrlMap[elString] = make([]string, 0)
			}
			uaUrlMap[elString] = append(uaUrlMap[elString], nowElUrlPath.String())
		}
	})
	if err != nil {
		return err
	}

	for uaName, uaUrls := range uaUrlMap {

		uaResultMap[uaName] = make([]string, 0)
		for index, uaUrl := range uaUrls {

			logger.Infoln(uaName, index, uaUrl)
			nowPage, p, err = PageNavigate(nowPage, false, uaUrl, 15*time.Second)
			if err != nil {
				return err
			}
			StatusCodeCheck, err = PageStatusCodeCheck(
				p,
				[]StatusCodeInfo{statusCode})
			if err != nil {
				return err
			}
			switch StatusCodeCheck {
			case Skip, Repeat:
				// 跳过后续的逻辑，不需要再次访问
				return errors.New("StatusCodeCheck Error")
			}
			pageLoaded = HasPageLoaded(nowPage, pageAllXPath, 15)
			if pageLoaded == false {
				return errors.New("HasPageLoaded == false")
			}
			err = rod.Try(func() {
				// 遍历所有的 ul ,然后再次遍历 ul 中的 A 的链接
				aULs := nowPage.MustElements("ul")
				for _, ul := range aULs {

					aEls := ul.MustElements("a")
					for _, aEl := range aEls {

						uaString := aEl.MustText()
						uaResultMap[uaName] = append(uaResultMap[uaName], uaString)
					}
				}
			})
			if err != nil {
				return err
			}
		}
	}
	// 当前的目录下缓存下来
	saveRootPath := filepath.Join(".", "cache", "ua")
	if IsDir(saveRootPath) == false {
		err = os.MkdirAll(saveRootPath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	// 根据查询到的结果，写入本地的缓存
	for uaName, results := range uaResultMap {
		nowInfo := UserAgentInfo{
			UserAgentMainType: Browsers,
			SubType:           uaName,
			UserAgents:        results,
		}

		desSaveFPath := filepath.Join(saveRootPath, uaName+".json")
		logger.Infoln("uaName:", uaName, desSaveFPath)
		err = ToFile(desSaveFPath, nowInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

func RandomUserAgent() string {

	// 是否已经读取过本地的缓存
	if len(allUANames) > 0 {
		return allUANames[rand.Intn(len(allUANames))]
	} else {
		logger.Panicln("RandomUserAgent is empty, Need Call InitFakeUA()")
	}

	return ""
}

type UserAgentInfo struct {
	UserAgentMainType UserAgentMainType // 主要的分类
	SubType           string            // 比如是浏览器的分类中，可以是 Chrome 或者是 Firefox
	UserAgents        []string          // 这个子分类中有那些 UserAgent
}

type UserAgentMainType int

const (
	All UserAgentMainType = iota + 1
	Crawlers
	Browsers
	MobileBrowsers
	Consoles
	OfflineBrowsers
	EMailClients
	LinkCheckers
	EMailCollectors
	Validators
	FeedReaders
	Libraries
	CloudPlatforms
	Ohters
)

const (
	Chrome  = "Chrome"
	Edge    = "Edge"
	Firefox = "Firefox"
	Opera   = "Opera"
	Safari  = "Safari"
	Mozilla = "Mozilla"
)

func isSupportUAName(inName string) bool {

	switch inName {
	case Chrome, Edge, Firefox, Opera, Safari, Mozilla:
		return true
	}
	return false
}

var (
	allUANames []string
	subTypes   = []string{
		Chrome,
		Edge,
		Firefox,
		Opera,
		Safari,
		Mozilla,
	}
)

var browserUAs  [][]byte

var (
	//go:embed assets/ua/Chrome.json
	ChromeJson []byte

	//go:embed assets/ua/Edge.json
	EdgeJson []byte

	//go:embed assets/ua/Firefox.json
	FirefoxJson []byte

	//go:embed assets/ua/Opera.json
	OperaJson []byte

	//go:embed assets/ua/Safari.json
	SafariJson []byte

	//go:embed assets/ua/Mozilla.json
	MozillaJson []byte
)
