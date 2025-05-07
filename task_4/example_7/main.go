package main

import "github.com/gin-gonic/gin"

func main() {
	// 禁止日志的颜色
	gin.DisableConsoleColor()

	// 用默认中间件创建一个 gin 路由:
	// 日志和恢复（无崩溃）中间件
	router := gin.Default()

	router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	router.Run(":8080")
}

//func main() {
//	// 强制日志颜色化
//	gin.ForceConsoleColor()
//
//	// 用默认中间件创建一个 gin 路由:
//	// 日志和恢复（无崩溃）中间件
//	router := gin.Default()
//
//	router.GET("/ping", func(c *gin.Context) {
//		c.String(200, "pong")
//	})
//
//	router.Run(":8080")
//}
