package main

import (
	"github.com/gin-gonic/gin"
)

type User struct {
	Username string `json:"username"`
	Gender   string `json:"gender"`
}

func setupRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})
	return router
}

func postUser(router *gin.Engine) *gin.Engine {
	router.POST("/user/add", func(c *gin.Context) {
		var user User
		c.BindJSON(&user)
		c.JSON(200, user)
	})
	return router
}

func main() {
	router := setupRouter()
	router = postUser(router)
	router.Run(":8080")
}
