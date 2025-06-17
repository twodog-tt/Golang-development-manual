package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func main() {
	router := gin.Default()

	router.GET("/JSONP", func(c *gin.Context) {
		data := map[string]interface{}{
			"foo": "bar",
		}

		// /JSONP?callback=x
		// 将输出：x({\"foo\":\"bar\"})
		c.JSONP(http.StatusOK, data)
	})

	// 监听并在 0.0.0.0:8080 上启动服务
	router.Run(":8080")
}

// 虽然现代应用可能更倾向于使用 CORS，但了解 JSONP 仍有助于处理某些特定场景的需求。
// 适用场景:需要支持老旧浏览器,快速实现跨域数据获取,第三方数据嵌入（如天气插件）
