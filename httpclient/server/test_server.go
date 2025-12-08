package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 创建 Gin 引擎（生产环境用 gin.ReleaseMode）
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 2. 定义路由（示例接口）
	r.GET("/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message":  "HTTPS 服务正常运行",
			"protocol": c.Request.Proto, // 验证协议为 HTTP/1.1 或 HTTP/2
		})
	})

	// 3. 启动 HTTPS 服务（证书路径 + 私钥路径）
	// 监听 443 端口（HTTPS 标准端口，需 root 权限；开发环境可改用 8443 端口避免权限问题）
	certFile := "../../network/cert1/server.crt" // 你的证书路径
	keyFile := "../../network/cert1/server.key"  // 你的私钥路径
	err := r.RunTLS(":4443", certFile, keyFile)
	if err != nil {
		panic("HTTPS 服务启动失败：" + err.Error())
	}
}
