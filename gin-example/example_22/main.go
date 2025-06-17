package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func main() {
	router := gin.Default()

	// 此 handler 将匹配 /user/john 但不会匹配 /user/ 或者 /user
	router.GET("/user/:name", func(c *gin.Context) {
		name := c.Param("name")
		c.String(http.StatusOK, "Hello %s", name)
	})

	// 此 handler 将匹配 /user/john/ 和 /user/john/send
	// 如果没有其他路由匹配 /user/john，它将重定向到 /user/john/
	router.GET("/user/:name/*action", func(c *gin.Context) {
		name := c.Param("name")
		action := c.Param("action")
		message := name + " is " + action
		c.String(http.StatusOK, message)
	})

	router.Run(":8080")
}

/*
curl "http://localhost:8080/user/john/fuck"
john is /fuck
*/
