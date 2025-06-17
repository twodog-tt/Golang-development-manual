package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()

		// 设置 example 变量 	 在请求处理前设置上下文值
		c.Set("example", "12345")

		// 请求前
		// 调用c.Next()执行后续处理器(包括路由处理器)
		c.Next()

		// 请求后
		// 请求处理完成后执行的代码
		latency := time.Since(t) // 计算请求耗时
		log.Print(latency)

		// 获取发送的 status
		status := c.Writer.Status() // 获取响应状态码
		log.Println(status)
	}
}

func main() {
	router := gin.New()  // 创建不带默认中间件的Gin路由器
	router.Use(Logger()) // 注册Logger中间件

	router.GET("/test", func(c *gin.Context) {
		example := c.MustGet("example").(string) // 从上下文中获取值

		// 打印："12345"
		log.Println(example)
	})

	// 监听并在 0.0.0.0:8080 上启动服务
	router.Run(":8080")
}

/*
2025/05/07 13:31:47 12345
2025/05/07 13:31:47 476.667µs
2025/05/07 13:31:47 200
*/
