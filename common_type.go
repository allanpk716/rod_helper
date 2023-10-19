package rod_helper

import (
	"sync"
	"time"
)

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

type FilterInfo struct {
	KeyName   string     // 这次目标网站的关键词
	PageInfos []PageInfo // 需要测试的 UrlInfo
}

func NewFilterInfo(key string, needTestUrlInfos []PageInfo) *FilterInfo {
	return &FilterInfo{KeyName: key, PageInfos: needTestUrlInfos}
}

type PageInfo struct {
	Name               string            // 这个页面的目标是干什么
	Url                string            // 这个页面的 Url
	PageTimeOut        int               // 这个页面加载的超时时间
	Header             map[string]string // 这个页面的 Header
	SuccessWord        []string          // 为空的时候无需检测
	ExistElementXPaths []string          // 必须存在的元素 XPath
}

func (p PageInfo) GetPageTimeOut() time.Duration {
	return time.Duration(p.PageTimeOut) * time.Second
}

func (p PageInfo) HasSuccessWord() bool {
	if p.SuccessWord != nil && len(p.SuccessWord) > 0 {
		return true
	} else {
		return false
	}
}

type DeliveryInfo struct {
	Browser   *BrowserInfo
	ProxyInfo *XrayPoolProxyInfo
	PageInfos []PageInfo
	Wg        *sync.WaitGroup
	LoadType  TryLoadType
}

// TryLoadType 测试连接的类型
type TryLoadType int

const (
	WebPageWithBrowser TryLoadType = iota + 1
	WebPageWithHttpClient
)

type ProxyCache struct {
	UpdateTime               map[string]int64 // 更新时间
	FilterProxyInfoIndexList map[string][]int // 过滤后的代理信息
	NowFilterProxyInfoIndex  map[string]int   // 过滤后的代理信息的索引
}

func NewProxyCache() *ProxyCache {
	pc := ProxyCache{
		FilterProxyInfoIndexList: make(map[string][]int),
		NowFilterProxyInfoIndex:  make(map[string]int),
		UpdateTime:               make(map[string]int64),
	}
	return &pc
}
