package rod_helper

import (
	"github.com/sirupsen/logrus"
)

type PoolOptions struct {
	Log                  *logrus.Logger     // 日志
	loadAdblock          bool               // 是否加载 adblock
	loadPic              bool               // 是否加载图片
	preLoadUrl           string             // 预加载的url
	xrayPoolUrl          string             // xray pool url
	xrayPoolPort         string             // xray pool port
	browserInstanceCount int                // 浏览器最大的实例，xrayPoolUrl 有值的时候生效，用于爬虫。因为每启动一个实例就试用一个固定的代理，所以需要多个才行
	cacheRootDirPath     string             // 缓存的根目录
	browserFPath         string             // 浏览器的路径
	timeConfig           TimeConfig         // 时间设置
	successWordsConfig   SuccessWordsConfig // 成功的关键词
	failWordsConfig      FailWordsConfig    // 失败的关键词
}

func NewPoolOptions(log *logrus.Logger, loadAdblock bool, loadPic bool, timeConfig TimeConfig) *PoolOptions {
	return &PoolOptions{
		Log:                  log,
		loadAdblock:          loadAdblock,
		loadPic:              loadPic,
		browserInstanceCount: 1,
		timeConfig:           timeConfig}
}

func (r *PoolOptions) SetPreLoadUrl(url string) {
	r.preLoadUrl = url
}

func (r *PoolOptions) PreLoadUrl() string {
	return r.preLoadUrl
}

// SetXrayPoolUrl 127.0.0.1
func (r *PoolOptions) SetXrayPoolUrl(xrayUrl string) {
	r.xrayPoolUrl = xrayUrl
}

// XrayPoolUrl 127.0.0.1
func (r *PoolOptions) XrayPoolUrl() string {
	return r.xrayPoolUrl
}

// SetXrayPoolPort 19038
func (r *PoolOptions) SetXrayPoolPort(xrayPort string) {
	r.xrayPoolPort = xrayPort
}

// XrayPoolPort 19038
func (r *PoolOptions) XrayPoolPort() string {
	return r.xrayPoolPort
}

func (r *PoolOptions) SetBrowserInstanceCount(count int) {
	r.browserInstanceCount = count
}
func (r *PoolOptions) BrowserInstanceCount() int {
	return r.browserInstanceCount
}

func (r *PoolOptions) SetLoadAdblock(loadAdblock bool)  {
	r.loadAdblock = loadAdblock
}

func (r *PoolOptions) LoadAdblock() bool {
	return r.loadAdblock
}

func (r *PoolOptions) LoadPicture() bool {
	return r.loadPic
}

func (r *PoolOptions) BrowserFPath() string {
	return r.browserFPath
}

func (r *PoolOptions) SetBrowserFPath(path string) {
	r.browserFPath = path
}

func (r *PoolOptions) CacheRootDirPath() string {
	return r.cacheRootDirPath
}

func (r *PoolOptions) SetCacheRootDirPath(path string) {
	r.cacheRootDirPath = path
}

func (r *PoolOptions) SetSuccessWordsConfig(successWordsConfig SuccessWordsConfig) {
	r.successWordsConfig = successWordsConfig
}

func (r *PoolOptions) GetSuccessWordsConfig() SuccessWordsConfig {
	return r.successWordsConfig
}

func (r *PoolOptions) SetFailWordsConfig(failWordsConfig FailWordsConfig) {
	r.failWordsConfig = failWordsConfig
}

func (r *PoolOptions) GetFailWordsConfig() FailWordsConfig {
	return r.failWordsConfig
}

func (r *PoolOptions) GetTimeConfig() TimeConfig {
	return r.timeConfig
}
