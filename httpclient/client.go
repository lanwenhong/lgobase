package httpclient

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/lanwenhong/lgobase/logger"
)

func NewHttpClient(transport *http.Transport) *resty.Client {
	client := resty.New()
	if transport == nil {
		DefaultTransport := &http.Transport{
			//连接池配置
			MaxIdleConns:        100,              // 最大空闲连接数
			MaxIdleConnsPerHost: 20,               // 每个主机的最大空闲连接数
			IdleConnTimeout:     30 * time.Second, // 空闲连接超时时间（超过则关闭）

			// 3. 其他 Transport 配置
			MaxConnsPerHost:       50,               // 每个主机的最大并发连接数
			ResponseHeaderTimeout: 10 * time.Second, // 等待响应头的超时时间
			ExpectContinueTimeout: 1 * time.Second,  // 发送 Expect: 100-continue 后的超时时间
		}

		client.SetTransport(DefaultTransport)
	} else {
		client.SetTransport(transport)
	}

	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		s := ""
		switch req.Body.(type) {
		case string:
			s = req.Body.(string)
		default:
			s = "NonStringBody"
		}
		ctx := req.Context()
		logger.Infof(ctx, "send|url=%s|body=%s", req.URL, s)
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		ctx := resp.Request.Context()
		costTime := resp.Time()
		logger.Infof(ctx, "recv|method=%s|url=%s|code=%d|ret=%s|time=%dms", resp.Request.Method, resp.Request.URL, resp.StatusCode(), resp.String(), costTime.Milliseconds())
		return nil
	})
	return client
}

func ClientSetTlsConf(client *resty.Client, tslConfig *tls.Config) {
	client.SetTLSClientConfig(tslConfig)
}
