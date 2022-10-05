package rod_helper

import (
	"testing"
)

func TestNewBrowserBase(t *testing.T) {

	httpProxyUrl := "http://192.168.50.233:10807"
	_, err := NewBrowserBase(httpProxyUrl, true)
	if err != nil {
		t.Fatal(err)
	}
}
