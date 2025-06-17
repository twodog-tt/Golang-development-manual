package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.GET("/long_async", func(c *gin.Context) {
		// 创建在 goroutine 中使用的副本
		cCp := c.Copy()
		go func() {
			// 用 time.Sleep() 模拟一个长任务。
			time.Sleep(5 * time.Second)

			// 请注意您使用的是复制的上下文 "cCp"，这一点很重要
			log.Println("Done! in path " + cCp.Request.URL.Path)
		}()
	})

	router.GET("/long_sync", func(c *gin.Context) {
		// 用 time.Sleep() 模拟一个长任务。
		time.Sleep(5 * time.Second)

		// 因为没有使用 goroutine，不需要拷贝上下文
		log.Println("Done! in path " + c.Request.URL.Path)
	})

	// 监听并在 0.0.0.0:8080 上启动服务
	router.Run(":8080")
}

/*
[GIN] 2025/05/07 - 13:51:16 | 200 |       3.583µs |             ::1 | GET      "/long_async"
2025/05/07 13:51:21 Done! in path /long_async
[GIN] 2025/05/07 - 13:51:39 | 200 |  5.001116167s |             ::1 | GET      "/long_sync"
2025/05/07 13:51:39 Done! in path /long_sync
*/
