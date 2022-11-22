package rod_helper

import (
	"testing"
)

func TestNewBrowserBase(t *testing.T) {

	httpProxyUrl := "http://127.0.0.1:10809"
	//movieUrl := "https://www.google.com"
	b, err := NewBrowserBase(GetRodTmpRootFolder(), "", httpProxyUrl, true, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewPage(b.Browser)
	if err != nil {
		return
	}
	println(b)
}
