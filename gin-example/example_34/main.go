package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func main() {
	router := gin.Default()
	// 为 multipart forms 设置较低的内存限制 (默认是 32 MiB)
	router.MaxMultipartMemory = 8 << 20 // 8 MiB
	router.POST("/upload", func(c *gin.Context) {
		// Multipart form
		form, _ := c.MultipartForm()
		files := form.File["upload[]"]

		for _, file := range files {
			log.Println(file.Filename)

			// 上传文件至指定目录
			c.SaveUploadedFile(file, "./files/"+file.Filename)
		}
		c.String(http.StatusOK, fmt.Sprintf("%d files uploaded!", len(files)))
	})
	router.Run(":8080")
}

//func main() {
//	router := gin.Default()
//	// 为 multipart forms 设置较低的内存限制 (默认是 32 MiB)
//	router.MaxMultipartMemory = 8 << 20  // 8 MiB
//	router.POST("/upload", func(c *gin.Context) {
//		// 单文件
//		file, _ := c.FormFile("file")
//		log.Println(file.Filename)
//
//		dst := "./" + file.Filename
//		// 上传文件至指定的完整文件路径
//		c.SaveUploadedFile(file, "./files/" + file.Filename)
//
//		c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
//	})
//	router.Run(":8080")
//}
