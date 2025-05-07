package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default() // 创建默认Gin路由器(带Logger和Recovery中间件)

	s := &http.Server{
		Addr:           ":8080",          // 监听地址和端口
		Handler:        router,           // 使用Gin作为请求处理器
		ReadTimeout:    10 * time.Second, // 读取超时时间
		WriteTimeout:   10 * time.Second, // 写入超时时间
		MaxHeaderBytes: 1 << 20,          // 最大请求头大小(1MB)
	}
	err := s.ListenAndServe()
	if err != nil {
		return
	}
}
