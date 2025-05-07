package main

import "github.com/gin-gonic/gin"

func main() {
	router := gin.Default()

	// 提供 unicode 实体
	router.GET("/json", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"html": "<b>Hello, world!</b>",
		})
	})

	// 提供字面字符
	router.GET("/purejson", func(c *gin.Context) {
		c.PureJSON(200, gin.H{
			"html": "<b>Hello, world!</b>",
		})
	})

	// 监听并在 0.0.0.0:8080 上启动服务
	router.Run(":8080")
}

/*
   curl "http://localhost:8080/json"
   特殊字符会被转义为 Unicode 实体
   {"html":"\u003cb\u003eHello, world!\u003c/b\u003e"}

   curl "http://localhost:8080/purejson"
   保持原始字符，不进行转义
   {"html":"<b>Hello, world!</b>"}
*/
