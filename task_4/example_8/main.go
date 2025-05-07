package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {

	router := gin.Default()

	router.GET("/cookie", func(c *gin.Context) {

		cookie, err := c.Cookie("gin_cookie")

		if err != nil {
			cookie = "NotSet"
			// 删除cookie: 通过设置max age的值为-1.
			//c.SetCookie("gin_cookie", "test", -1, "/", "localhost", false, true)
			c.SetCookie("gin_cookie", "test", 3600, "/", "localhost", false, true)
		}

		fmt.Printf("Cookie value: %s \n", cookie)
		// Cookie value: test
	})

	router.Run()
}
