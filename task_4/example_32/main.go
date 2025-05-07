package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	// 作用：将本地目录 ./assets 映射为 URL 路径 /assets。
	// 示例：http://localhost:8080/assets/logo.png 实际返回的是 ./assets/logo.png 文件。
	router.Static("/assets", "./assets")

	// 作用：类似上面的 Static，但可以使用 http.FileSystem 接口，允许更灵活的文件系统接入。
	// 示例：http://localhost:8080/more_static/file.txt 实际访问 ./my_file_system/file.txt。
	// 使用 http.Dir(...) 是构造一个实现了 http.FileSystem 接口的本地目录。
	router.StaticFS("/more_static", http.Dir("my_file_system"))

	// 作用：访问 http://localhost:8080/favicon.ico 时返回的是 ./resources/favicon.ico 文件。
	// 用于提供网站图标等固定文件。
	router.StaticFile("/favicon.ico", "./resources/favicon.ico")

	// 监听并在 0.0.0.0:8080 上启动服务
	router.Run(":8080")
}
