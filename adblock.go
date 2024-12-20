package rod_helper

import (
	"fmt"
	"github.com/WQGroup/logger"
	"github.com/go-resty/resty/v2"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/mediabuyerbot/go-crx3"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var nowBlocker = uBlock

// SwitchAdBlocker 切换广告屏蔽的核心插件是用那个
func SwitchAdBlocker(which BlockerType) {
	nowBlocker = which
}

// GetADBlock 根据缓存时间，每周获取一次最新的 adblock，注意需要完全关闭所有的 browser，再进行次操作
func GetADBlock(cacheRootDirPath, httpProxyUrl string) (string, error) {

	defer func() {
		logger.Infoln("get adblock done")
	}()
	nowUserData := filepath.Join(GetRodTmpRootFolder(cacheRootDirPath), RandStringBytesMaskImprSrcSB(20))
	purl := launcher.New().
		UserDataDir(nowUserData).
		MustLaunch()
	var browser *rod.Browser
	browser = rod.New().ControlURL(purl).MustConnect()
	defer func() {
		if browser != nil {
			_ = browser.Close()
		}

		time.AfterFunc(time.Second*5, func() {

			err := os.RemoveAll(nowUserData)
			if err != nil {
				logger.Errorln("clear rod tmp root folder failed: ", err)
			}
		})
	}()
	vResult, err := browser.Version()
	if err != nil {
		return "", errors.New(fmt.Sprintf("get browser version failed: %s", err))
	}
	browserVersion := vResult.Product
	versions := strings.Split(browserVersion, "/")
	if len(versions) != 2 {
		return "", errors.New("Chrome Version: " + browserVersion + " Can't split by '/'")
	}
	browserVersion = versions[1]
	logger.Infoln("browser version: ", browserVersion)
	// 判断插件是否已经下载
	desFile := filepath.Join(GetADBlockFolder(cacheRootDirPath), browserVersion+".crx")
	if IsFile(desFile) == false {
		// || getDownloadedCacheTime(cacheRootDirPath).DownloadedTime < time.Now().AddDate(0, 0, -7).Unix()
		// 没有下载，那么就去下载，或者下载的时间超过了一周，也需要再次下载
		// 下载插件
		logger.Infoln("download adblock plugin start...")
		client := resty.New()
		if httpProxyUrl != "" {
			client.SetProxy(httpProxyUrl)
		}
		client.SetTimeout(1 * time.Minute)
		client.SetOutputDirectory(GetADBlockFolder(cacheRootDirPath))

		tmpBlockID := uBlockID
		if nowBlocker == uBlock {
			tmpBlockID = uBlockID
		} else {
			tmpBlockID = adblockID
		}
		adblockDownloadUrl := adblockDownloadUrl0 + browserVersion + adblockDownloadUrl1 + tmpBlockID + adblockDownloadUrl2
		_, err = client.R().
			SetOutput(browserVersion + ".crx").
			Get(adblockDownloadUrl)
		if err != nil {
			return "", err
		}

		if IsFile(desFile) == false {
			return "", errors.New("get adblock from web failed")
		}

		setDownloadedCacheTime(cacheRootDirPath,
			&ADBlockCacheInfo{
				DownloadedTime: time.Now().Unix(),
			})
	}

	return desFile, nil
}

// GetADBlockLocalPath 获取本地的 adblock 插件路径，如果不存在会自动去远程下载
func GetADBlockLocalPath(cacheRootDirPath, httpProxyUrl string) string {

	var err error
	var desFile string
	for i := 1; i <= 5; i++ {

		logger.Infoln("get adblock local path start... try:", i, "time")
		desFile, err = GetADBlock(cacheRootDirPath, httpProxyUrl)
		if err != nil {
			logger.Errorln(fmt.Sprintf("get adblock failed %d tims: %s", i, err))
			continue
		}

		if err = crx3.Extension(desFile).Unpack(); err != nil {
			logger.Errorln(fmt.Sprintf("unpack adblock failed %d tims: %s", i, err.Error()))
			logger.Infoln("try to remove adblock file: ", desFile)
			if IsFile(desFile) == true {
				_ = os.Remove(desFile)
			}
			continue
		}
		break
	}
	if err != nil {
		logger.Panicln("GetADBlockLocalPath: ", err)
	}

	filenameOnly := strings.TrimSuffix(filepath.Base(desFile), filepath.Ext(desFile))

	return filepath.Join(GetADBlockFolder(cacheRootDirPath), filenameOnly)
}

func getDownloadedCacheTime(cacheRootDirPath string) *ADBlockCacheInfo {

	saveFPath := filepath.Join(GetADBlockFolder(cacheRootDirPath), adblockCacheFileName)
	if IsFile(saveFPath) == false {
		// 需要保存一个新的
		info := ADBlockCacheInfo{
			DownloadedTime: time.Now().Unix(),
		}
		err := ToFile(saveFPath, info)
		if err != nil {
			logger.Panicln("save adblock cache info failed: ", err)
		}
		return &info
	} else {
		// 如果存在，那么就直接读取
		info := ADBlockCacheInfo{}
		err := ToStruct(saveFPath, &info)
		if err != nil {
			logger.Panicln("read adblock cache info failed: ", err)
		}
		return &info
	}
}

func setDownloadedCacheTime(cacheRootDirPath string, info *ADBlockCacheInfo) {
	saveFPath := filepath.Join(GetADBlockFolder(cacheRootDirPath), adblockCacheFileName)
	err := ToFile(saveFPath, *info)
	if err != nil {
		logger.Panicln("save adblock cache info failed: ", err)
	}
}

type ADBlockCacheInfo struct {
	DownloadedTime int64
}

const adblockDownloadUrl0 = "https://clients2.google.com/service/update2/crx?response=redirect&prodversion="
const adblockDownloadUrl1 = "&acceptformat=crx2%2Ccrx3&x=id%3D"
const adblockDownloadUrl2 = "%26uc"
const adblockID = "gighmmpiobklfepjocnamgkkbiglidom"
const adblockCacheFileName = "cache_time.json"

const uBlockID = "cjpalhdlnbpafiamejdnhcphjbkeiagm"

type BlockerType int

const (
	AdBlock BlockerType = iota
	uBlock
)
