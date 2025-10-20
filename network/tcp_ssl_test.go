package network

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/network"
	"github.com/lanwenhong/lgobase/util"
)

func TestSSlSend(t *testing.T) {
	ctx := context.WithValue(context.Background(), "trace_id", util.GenXid())
	rootCAData, err := os.ReadFile("root.crt")
	if err != nil {
		t.Fatal(err)
	}

	rootCAs := x509.NewCertPool()
	// 将 PEM 格式的根证书添加到证书池
	succ := rootCAs.AppendCertsFromPEM(rootCAData)
	if succ != true {
		t.Fatal(succ)
	}

	tlsConfig := &tls.Config{
		RootCAs:    rootCAs,                          // 信任的根 CA 池（用于验证服务器证书）
		ServerName: "terminal.uat.planetpayment.com", // 服务器证书的域名（必须与证书的 CN 或 SAN 匹配）
		MinVersion: tls.VersionTLS12,                 // 禁用不安全的 TLS 版本
		// 禁用 InsecureSkipVerify（生产环境绝对不能开启！）
		//InsecureSkipVerify: true, // 开启后会跳过证书验证（不安全）
	}

	cTimeout := 3 * time.Second
	rTimeout := 30 * time.Second
	wTimeout := 30 * time.Second

	c := network.NewTcpSsslConn("terminal.uat.planetpayment.com:40860", cTimeout, rTimeout, wTimeout, tlsConfig)
	err = c.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bcd := "0094600010000002003020058020C0900200200000000000055500002700210226003750100439999991007D0810120000000323701F31313431313131313138383030303334343333332020200124AA17EAB7BF18034B0061000331325800044941022000064942463945410003494303000349440100034945000003494600000349470000034948000010494C0000702940000850"
	b, _ := hex.DecodeString(bcd)
	c.Writen(ctx, b)

	head, err := c.Readn(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}

	slen := hex.EncodeToString(head)
	logger.Debugf(ctx, "slen: %s", slen)
	blen, err := strconv.ParseUint(slen, 16, 32)
	if err != nil {
		logger.Warnf(ctx, "err: %s", err.Error())
		return
	}
	logger.Debugf(ctx, "blen: %d", blen)
	data, err := c.Readn(ctx, int(blen))

	if err != nil {
		t.Fatal(err)
	}
	bcdData := hex.EncodeToString(data)
	logger.Debugf(ctx, "bcdData: %s", bcdData)

}

func TestSSlSendMulti(t *testing.T) {

	ctx := context.WithValue(context.Background(), "trace_id", util.GenXid())
	rootCAData, err := os.ReadFile("root.crt")
	if err != nil {
		t.Fatal(err)
	}

	rootCAs := x509.NewCertPool()
	// 将 PEM 格式的根证书添加到证书池
	succ := rootCAs.AppendCertsFromPEM(rootCAData)
	if succ != true {
		t.Fatal(succ)
	}

	tlsConfig := &tls.Config{
		RootCAs:    rootCAs,                          // 信任的根 CA 池（用于验证服务器证书）
		ServerName: "terminal.uat.planetpayment.com", // 服务器证书的域名（必须与证书的 CN 或 SAN 匹配）
		MinVersion: tls.VersionTLS12,                 // 禁用不安全的 TLS 版本
		// 禁用 InsecureSkipVerify（生产环境绝对不能开启！）
		//InsecureSkipVerify: true, // 开启后会跳过证书验证（不安全）
	}

	cTimeout := 3 * time.Second
	rTimeout := 30 * time.Second
	wTimeout := 30 * time.Second

	logger.Debugf(ctx, "gogo")
	for i := 0; i <= 4; i++ {
		ctx := context.WithValue(context.Background(), "trace_id", util.GenXid())
		go func() {
			c := network.NewTcpSsslConn("terminal.uat.planetpayment.com:40860", cTimeout, rTimeout, wTimeout, tlsConfig)
			err = c.Open(ctx)
			if err != nil {
				t.Fatal(err)
			}
			bcd := "0094600010000002003020058020C0900200200000000000055500002700210226003750100439999991007D0810120000000323701F31313431313131313138383030303334343333332020200124AA17EAB7BF18034B0061000331325800044941022000064942463945410003494303000349440100034945000003494600000349470000034948000010494C0000702940000850"
			b, _ := hex.DecodeString(bcd)
			c.Writen(ctx, b)

			head, err := c.Readn(ctx, 2)
			if err != nil {
				t.Fatal(err)
			}

			slen := hex.EncodeToString(head)
			logger.Debugf(ctx, "slen: %s", slen)
			blen, err := strconv.ParseUint(slen, 16, 32)
			if err != nil {
				logger.Warnf(ctx, "err: %s", err.Error())
				return
			}
			logger.Debugf(ctx, "blen: %d", blen)
			data, err := c.Readn(ctx, int(blen))

			if err != nil {
				t.Fatal(err)
			}
			bcdData := hex.EncodeToString(data)
			logger.Debugf(ctx, "bcdData: %s", bcdData)
		}()
	}

	time.Sleep(10 * time.Second)

}
