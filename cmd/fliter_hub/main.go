package main

import (
	"fmt"
	"github.com/WQGroup/logger"
	"github.com/allanpk716/rod_helper"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
)

var pool *rod_helper.Pool
var httpPort int = 8080

func main() {

	pool = rod_helper.NewPool(rod_helper.NewPoolOptions(logger.GetLogger(),
		true,
		false,
		rod_helper.TimeConfig{
			OnePageTimeOut:                 15,
			OneProxyNodeUseInternalMinTime: 30,
			OneProxyNodeUseInternalMaxTime: 45,
			ProxyNodeSkipAccessTime:        86400,
		}))
	if pool == nil {
		logger.Panic("pool is nil, xray_pool not running")
	}

	// -----------------------------------------
	// 设置跨域
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = ioutil.Discard
	engine := gin.Default()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: engine,
	}

	go func() {
		logger.Infoln("Try Start Http Server At Port", httpPort)
		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) == false {
			logger.Errorln("Start Server Error:", err)
		}
	}()

	// 阻塞
	select {}
}

// AddFilterTaskHandler 添加一个任务
func AddFilterTaskHandler(c *gin.Context) {

}

// ListFilterTasksHandler 列出所有的任务
func ListFilterTasksHandler(c *gin.Context) {

}

// GetFilterTaskStatusHandler 查询一个任务的状态
func GetFilterTaskStatusHandler(c *gin.Context) {

}
