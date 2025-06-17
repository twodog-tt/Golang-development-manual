package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	/*
		httpMethod：HTTP 方法（GET/POST 等）

		absolutePath：路由路径

		handlerName：处理函数名称

		nuHandlers：该路由的中间件数量
	*/
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		log.Printf("endpoint %v %v %v %v\n", httpMethod, absolutePath, handlerName, nuHandlers)
	}
	/*
		2025/05/07 13:46:09 endpoint POST /foo main.main.func2 3
		2025/05/07 13:46:09 endpoint GET /bar main.main.func3 3
		2025/05/07 13:46:09 endpoint GET /status main.main.func4 3
	*/

	router.POST("/foo", func(c *gin.Context) {
		c.JSON(http.StatusOK, "foo")
	})

	router.GET("/bar", func(c *gin.Context) {
		c.JSON(http.StatusOK, "bar")
	})

	router.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, "ok")
	})

	// 监听并在 0.0.0.0:8080 上启动服务
	router.Run()
}
