package rod_helper

import "github.com/pkg/errors"

var (
	ErrProxyInfosIsEmpty = errors.New("proxyInfos is empty")
	ErrSkipAccessTime    = errors.New("skipAccessTime")
	ErrIndexIsOutOfRange = errors.New("index is out of range")
)
