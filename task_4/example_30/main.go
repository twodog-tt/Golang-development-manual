package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	expectedHost := "localhost:8080"

	// Setup Security Headers
	r.Use(func(c *gin.Context) {
		// 如果请求的 Host 不符合预期，直接返回 400 错误并终止请求。 这是一种基础的安全检查，可以防止 Host header injection 等攻击
		if c.Request.Host != expectedHost {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid host header"})
			return
		}
		//防止网页被嵌入到 iframe 中，避免 Clickjacking 攻击。
		c.Header("X-Frame-Options", "DENY")
		// 设置内容安全策略（CSP），限制资源加载来源，提高 XSS 防御能力。
		c.Header("Content-Security-Policy", "default-src 'self'; connect-src *; font-src *; script-src-elem * 'unsafe-inline'; img-src * data:; style-src * 'unsafe-inline';")
		// 启用浏览器的 XSS 过滤器（老旧浏览器支持）。
		c.Header("X-XSS-Protection", "1; mode=block")
		// 强制浏览器使用 HTTPS 连接（HSTS）
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		// 限制 Referer 头的发送内容，保护隐私
		c.Header("Referrer-Policy", "strict-origin")
		// 阻止浏览器对 MIME 类型的自动嗅探，防止内容混淆攻击。
		c.Header("X-Content-Type-Options", "nosniff")
		// 限制浏览器访问某些特性（如麦克风、摄像头、全屏等）。
		c.Header("Permissions-Policy", "geolocation=(),midi=(),sync-xhr=(),microphone=(),camera=(),magnetometer=(),gyroscope=(),fullscreen=(self),payment=()")
		// 继续处理后续的中间件和请求处理逻辑。
		c.Next()
	})

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.Run() // listen and serve on 0.0.0.0:8080
}

/*
curl "localhost:8080/ping"
{"message":"pong"}
*/
