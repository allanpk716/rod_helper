package rod_helper

import (
	"testing"
	"time"
)

func TestNewBrowserBase(t *testing.T) {

	httpProxyUrl := "http://127.0.0.1:10809"
	movieUrl := "https://www.google.com"
	b, err := NewBrowserBase("", httpProxyUrl, true, false)
	if err != nil {
		t.Fatal(err)
	}
	page, _, err := NewPageNavigateWithProxy(b, httpProxyUrl, movieUrl, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	println(page.MustHTML())
}
