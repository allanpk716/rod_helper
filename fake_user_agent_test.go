package rod_helper

import "testing"

func TestGetFakeUserAgentDataCache(t *testing.T) {

	err := GetFakeUserAgentDataCache("C:\\Tmp", "http://192.168.50.252:20171")
	if err != nil {
		t.Fatal(err)
	}
}
