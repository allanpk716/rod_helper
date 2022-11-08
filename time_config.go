package rod_helper

import "time"

type TimeConfig struct {
	OneProxyNodeUseInternalMinTime int32 // 一个代理节点，两次使用最短间隔，单位是秒
	OneProxyNodeUseInternalMaxTime int32 // 一个代理节点，两次使用最长间隔，单位是秒
	ProxyNodeSkipAccessTime        int64 // 设置一个代理节点可被再次访问的时间间隔（然后需要再加上现在时间为基准来算），单位是秒
}

// GetOneProxyNodeUseInternalTime 获取一个代理节点，两次使用的间隔时间，单位是秒
func (t *TimeConfig) GetOneProxyNodeUseInternalTime(passTime int32) time.Duration {
	return RandomSecondDuration(t.OneProxyNodeUseInternalMinTime-passTime, t.OneProxyNodeUseInternalMaxTime-passTime)
}

// GetProxyNodeSkipAccessTime 这里返回的时候已经加上了当前的 unix time
func (t *TimeConfig) GetProxyNodeSkipAccessTime() int64 {
	return time.Now().Unix() + t.ProxyNodeSkipAccessTime
}
