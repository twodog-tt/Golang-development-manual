package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.POST("/post", func(c *gin.Context) {

		id := c.Query("id")
		page := c.DefaultQuery("page", "0")
		name := c.PostForm("name")
		message := c.PostForm("message")

		fmt.Printf("id: %s; page: %s; name: %s; message: %s", id, page, name, message)
	})
	router.Run(":8080")
}

/*
curl -X POST "http://localhost:8080/post?id=123&page=2"   -d "name=Alice&message=HelloWorld"
id: 123; page: 2; name: Alice; message: HelloWorld[GIN] 2025/05/07 - 15:49:53 | 200 |     172.959µs |             ::1 | POST     "/post?id=123&page=2"
*/
