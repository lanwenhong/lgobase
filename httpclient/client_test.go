package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"testing"

	"github.com/lanwenhong/lgobase/httpclient"
	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/util"
)

func TestHttpGet(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenXid())
	client := httpclient.NewHttpClient(nil)
	caCertPath := "../network/cert1/server.crt" // 你的 CA 根证书路径（PEM 格式）
	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		//log.Fatalf("读取 CA 证书失败：%v", err)
		logger.Warnf(ctx, "读取 CA 证书失败：%v", err)
		t.Fatal(err)
	}

	// 创建证书池，添加自定义 CA
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		//log.Fatalf("解析 CA 证书失败：无效的 PEM 格式")
		logger.Warnf(ctx, "解析 CA 证书失败：无效的 PEM 格式")
		t.Fatal(err)
	}
	TLSClientConfig := &tls.Config{
		RootCAs:            caCertPool,       // 信任自定义 CA
		MinVersion:         tls.VersionTLS12, // 禁用 TLS 1.0/1.1，最低支持 TLS 1.2
		MaxVersion:         tls.VersionTLS13, // 最高支持 TLS 1.3
		InsecureSkipVerify: false,            // 生产环境必须禁用（启用证书校验）
		// 若需双向认证，添加客户端证书：
		// Certificates: []tls.Certificate{clientCert}, // 需先通过 tls.LoadX509KeyPair 加载
	}
	httpclient.ClientSetTlsConf(client, TLSClientConfig)
	resp, err := client.R().SetContext(ctx).Get("https://localhost:4443/hello")
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
		t.Fatal(err)
	}

	logger.Debugf(ctx, "code: %d", resp.StatusCode())
}
