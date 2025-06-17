package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.POST("/post", func(c *gin.Context) {

		ids := c.QueryMap("ids") // 获取查询参数中的映射

		names := c.PostFormMap("names") // 获取表单数据中的映射

		fmt.Printf("ids: %v; names: %v", ids, names)
	})
	router.Run(":8080")
}
