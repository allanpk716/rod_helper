package rod_helper

import (
	"github.com/WQGroup/logger"
	"testing"
)

func TestNewMultiBrowser(t *testing.T) {

	InitFakeUA(true, "", "")
	browserOptions := NewPoolOptions(logger.GetLogger(), true, true, TimeConfig{
		OnePageTimeOut: 15,
	})
	browserOptions.SetXrayPoolUrl("192.168.50.233")
	browserOptions.SetXrayPoolPort("19038")
	b := NewPool(browserOptions)

	fInfo := NewFilterInfo("imdb", []PageInfo{
		{
			Name:        "Most Popular Movies",
			Url:         "https://www.imdb.com/chart/moviemeter/",
			PageTimeOut: 15,
			SuccessWord: []string{"Most Popular Movies"},
			ExistElementXPaths: []string{
				`//*[@id="__next"]/main/div/div[3]/section/div/div[1]/div/div[2]/hgroup/h1`,
			},
		},
	})
	err := b.Filter(fInfo, 2, true)
	if err != nil {
		t.Fatal(err)
	}

	proxyInfos, err := b.GetFilterProxyInfos(fInfo.Key)
	if err != nil {
		t.Fatal(err)
	}
	for _, proxy := range proxyInfos {
		println(proxy.Index, proxy.Name)
	}
}
