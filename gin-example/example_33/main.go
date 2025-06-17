package main

import (
	"log"

	"github.com/gin-gonic/autotls"
	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	// Ping handler
	router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	log.Fatal(autotls.Run(router, "example1.com", "example2.com"))

	/*
		启动 HTTPS 服务，自动为 example1.com 和 example2.com 获取 TLS 证书；

		使用的是 Let's Encrypt 的 ACME 协议，首次启动会自动验证域名并下载证书；

		如果获取失败，log.Fatal 会输出错误并中止程序。
	*/
}
