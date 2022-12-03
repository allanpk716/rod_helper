package rod_helper

import "testing"

func TestGetADBlock(t *testing.T) {

	httpProxyUrl := "http://192.168.50.233:10807"
	_, err := GetADBlock("C:\\Tmp", httpProxyUrl)
	if err != nil {
		t.Fatal(err)
	}
}
