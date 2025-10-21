package network

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"os"
	"strconv"
	"testing"

	"github.com/lanwenhong/lgobase/logger"
	"github.com/lanwenhong/lgobase/network"
	"github.com/lanwenhong/lgobase/util"
)

func TestSSlPool(t *testing.T) {
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

	g_conf := &network.TcpPoolConfig[*network.TcpSslConn]{
		Addrs:        "terminal.uat.planetpayment.com:40860/7000",
		MaxConns:     100,
		MaxIdleConns: 10,
		MaxConnLife:  10,
		PurgeRate:    0.5,
		Cfunc:        network.NewSingleTcpConn[*network.TcpSslConn],
		TlsConf: &tls.Config{
			RootCAs:    rootCAs,                          // 信任的根 CA 池（用于验证服务器证书）
			ServerName: "terminal.uat.planetpayment.com", // 服务器证书的域名（必须与证书的 CN 或 SAN 匹配）
			MinVersion: tls.VersionTLS12,                 // 禁用不安全的 TLS 版本
			// 禁用 InsecureSkipVerify（生产环境绝对不能开启！）
			//InsecureSkipVerify: true, // 开启后会跳过证书验证（不安全）
		},
	}

	rps := network.TcpPoolSelector[*network.TcpSslConn]{}
	rps.RpcPoolInit(ctx, g_conf)

	process := func(conn interface{}) (string, error) {
		var err error = nil
		c := conn.(*network.TcpSslConn)
		logger.Debugf(ctx, "c: %v", c)

		bcd := "0094600010000002003020058020C0900200200000000000055500002700210226003750100439999991007D0810120000000323701F31313431313131313138383030303334343333332020200124AA17EAB7BF18034B0061000331325800044941022000064942463945410003494303000349440100034945000003494600000349470000034948000010494C0000702940000850"
		//bcd := "A0600010000008002020010000C000029200000004810226313134313131313131383830303033343433333320202000320010494C00007029400008500018505024504C414E4554245041594D454E5424"
		//bcd := "86080020380100000000029200005293231330570927005200370010494C000000000000000200034B54320018505024504C414E4554245041594D454E5424"
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
			return "echo", err
		}
		logger.Debugf(ctx, "blen: %d", blen)
		data, err := c.Readn(ctx, int(blen))

		if err != nil {
			return "echo", err
		}
		bcdData := hex.EncodeToString(data)
		logger.Debugf(ctx, "bcdData: %s", bcdData)

		return "echo", err
	}

	rps.Process(ctx, process)
}
