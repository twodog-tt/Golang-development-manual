package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	// 你也可以使用自己的 SecureJSON 前缀
	// router.SecureJsonPrefix(")]}',\n")

	router.GET("/someJSON", func(c *gin.Context) {
		names := []string{"lena", "austin", "foo"}

		// 将输出：while(1);["lena","austin","foo"]
		c.SecureJSON(http.StatusOK, names)
	})

	// 监听并在 0.0.0.0:8080 上启动服务
	router.Run(":8080")
}

/*
curl "localhost:8080/someJSON"
while(1);["lena","austin","foo"]
*/
