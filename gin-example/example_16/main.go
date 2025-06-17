package main

import "github.com/gin-gonic/gin"

func main() {
	// 禁用控制台颜色
	// gin.DisableConsoleColor()

	// 使用默认中间件（logger 和 recovery 中间件）创建 gin 路由
	router := gin.Default()

	router.GET("/someGet")
	router.POST("/somePost")
	router.PUT("/somePut")
	router.DELETE("/someDelete")
	router.PATCH("/somePatch")
	router.HEAD("/someHead")
	router.OPTIONS("/someOptions")

	// 默认在 8080 端口启动服务，除非定义了一个 PORT 的环境变量。
	router.Run()
	// router.Run(":3000") hardcode 端口号
}
