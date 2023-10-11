package rod_helper

import "github.com/pkg/errors"

var (
	ErrProxyInfosIsEmpty = errors.New("orgProxyInfos is empty")
	ErrSkipAccessTime    = errors.New("skipAccessTime")
	ErrIndexIsOutOfRange = errors.New("index is out of range")
	ErrPageLoadFailed    = errors.New("pageLoaded == false")
)
