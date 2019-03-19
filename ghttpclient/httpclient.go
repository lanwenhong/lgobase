package ghttpclient

import (
	"errors"
	"fmt"
	"github.com/lanwenhong/lgobase/logger"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	INTER_PROTO_PUSHAPI = iota
)

type UserHttpHeaderBuild func(req *http.Request, header map[string]string) error

type QfHttpClient struct {
	Protocol int
	Domain   string
	SslUse   bool
}

type HttpClientLong struct {
	Timeout int
	Client  *http.Client
	Uhhb    UserHttpHeaderBuild
}

type Qfresp struct {
	Status     string // e.g. "200 OK"
	StatusCode int    // e.g. 200
	Proto      string // e.g. "HTTP/1.0"
	ProtoMajor int    // e.g. 1
	ProtoMinor int    // e.g. 0
	Ret        []byte
	Header     http.Header
}

func NewHttpClient(timeout int) *HttpClientLong {
	c := new(HttpClientLong)
	c.Timeout = timeout
	c.Client = &http.Client{Timeout: time.Millisecond * time.Duration(timeout)}
	c.Uhhb = nil
	return c
}

func (c *HttpClientLong) setHeaderBuildFunc(f UserHttpHeaderBuild) {
	c.Uhhb = f
}

func (c *HttpClientLong) getResp(resp *http.Response) (r *Qfresp, err error) {
	r = new(Qfresp)
	r.Header = make(http.Header)
	r.Status = resp.Status
	r.StatusCode = resp.StatusCode
	r.Proto = resp.Proto
	r.ProtoMajor = resp.ProtoMajor
	r.ProtoMinor = resp.ProtoMinor
	r.Ret, err = ioutil.ReadAll(resp.Body)
	for k, v := range resp.Header {
		r.Header[k] = v
	}
	return r, err
}

func (c *HttpClientLong) realPost(qurl string, dreq interface{}, header map[string]string) (r *Qfresp, err error) {
	var body io.Reader

	if dreq == nil {
		return nil, errors.New("post data nil")
	}
	switch dreq.(type) {
	case map[string]string:
		v := url.Values{}
		for k, x := range dreq.(map[string]string) {
			v.Set(k, x)
		}
		body = ioutil.NopCloser(strings.NewReader(v.Encode()))
	case string:
		body = strings.NewReader(dreq.(string))
	default:
		return nil, errors.New("type not support")
	}
	req, _ := http.NewRequest("POST", qurl, body)
	if header != nil {
		if c.Uhhb != nil {
			c.Uhhb(req, header)
		} else {
			for k, v := range header {
				req.Header.Set(k, v)
			}
		}
	}
	resp, err := c.Client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		logger.Warnf("get err: %s", err.Error())
		return nil, err
	}
	return c.getResp(resp)
}

func HttpRealPost(curl string, timeout int32, dreq interface{}, header map[string]string) (ret []byte, err error) {
	var body io.Reader

	if dreq == nil {
		return nil, errors.New("post data nil")
	}
	client := http.Client{}
	client.Timeout = time.Duration(timeout) * time.Millisecond

	switch dreq.(type) {
	case map[string]string:
		v := url.Values{}
		for k, x := range dreq.(map[string]string) {
			v.Set(k, x)
		}
		body = ioutil.NopCloser(strings.NewReader(v.Encode()))
	case string:
		body = strings.NewReader(dreq.(string))
	default:
		return nil, errors.New("type not support")
	}
	req, _ := http.NewRequest("POST", curl, body)
	if header != nil {
		for k, v := range header {
			req.Header.Set(k, v)
		}
	}
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		logger.Warnf("get err: %s", err.Error())
		return nil, err
	}
	ret, err = ioutil.ReadAll(resp.Body)
	return ret, err
}

func (c *HttpClientLong) realGet(qurl string, dreq map[string]string, header map[string]string) (r *Qfresp, err error) {
	u, err := url.Parse(qurl)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if dreq != nil {
		for k, v := range dreq {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	req, _ := http.NewRequest("GET", u.String(), nil)
	if header != nil {
		if c.Uhhb != nil {
			c.Uhhb(req, header)
		} else {
			for k, v := range header {
				req.Header.Set(k, v)
			}
		}
	}
	resp, err := c.Client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		logger.Warnf("get err: %s", err.Error())
		return nil, err
	}
	return c.getResp(resp)
}

func HttpRealGet(curl string, timeout int32, dreq map[string]string, header map[string]string) (ret []byte, err error) {
	u, err := url.Parse(curl)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if dreq != nil {
		for k, v := range dreq {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	client := http.Client{}
	client.Timeout = time.Duration(timeout) * time.Millisecond
	req, _ := http.NewRequest("GET", u.String(), nil)
	if header != nil {
		for k, v := range header {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		logger.Warnf("get err: %s", err.Error())
		return nil, err
	}
	ret, err = ioutil.ReadAll(resp.Body)
	return ret, err
}

func QfHttpClientNew(protocol int, domain string, ssl_use bool) *QfHttpClient {
	qfh := new(QfHttpClient)
	qfh.Protocol = protocol
	qfh.Domain = domain
	qfh.SslUse = ssl_use

	return qfh
}

func (qfh *QfHttpClient) Get(path string, timeout int32, req map[string]string, header map[string]string) (ret []byte, err error) {
	var url = ""

	if qfh.SslUse {
		url = fmt.Sprintf("https://%s/%s", qfh.Domain, path)
	} else {
		url = fmt.Sprintf("http://%s/%s", qfh.Domain, path)
	}
	snow := time.Now()
	smicros := snow.UnixNano() / 1000
	ret, err = HttpRealGet(url, timeout, req, header)
	enow := time.Now()
	emicros := enow.UnixNano() / 1000
	logger.Infof("func=get|url=%s|req=%s|ret=%s|time=%d", url, req, ret, emicros-smicros)
	return ret, err
}

func (qfh *HttpClientLong) Getl(url string, req map[string]string, header map[string]string) (r *Qfresp, err error) {
	snow := time.Now()
	smicros := snow.UnixNano() / 1000
	r, err = qfh.realPost(url, req, header)
	enow := time.Now()
	emicros := enow.UnixNano() / 1000
	logger.Infof("func=post|url=%s|req=%s|ret=%s|time=%d", url, req, r.Ret, emicros-smicros)
	return r, err
}

func (qfh *QfHttpClient) Post(path string, timeout int32, req interface{}, header map[string]string) (ret []byte, err error) {
	var url = ""
	if qfh.SslUse {
		url = fmt.Sprintf("https://%s/%s", qfh.Domain, path)
	} else {
		url = fmt.Sprintf("http://%s/%s", qfh.Domain, path)
	}
	snow := time.Now()
	smicros := snow.UnixNano() / 1000
	ret, err = HttpRealPost(url, timeout, req, header)
	enow := time.Now()
	emicros := enow.UnixNano() / 1000
	logger.Infof("func=post|url=%s|req=%s|ret=%s|time=%d", url, req, ret, emicros-smicros)
	return ret, err
}

func (qfh *HttpClientLong) Postl(url string, req interface{}, header map[string]string) (r *Qfresp, err error) {
	snow := time.Now()
	smicros := snow.UnixNano() / 1000
	r, err = qfh.realPost(url, req, header)
	enow := time.Now()
	emicros := enow.UnixNano() / 1000
	logger.Infof("func=post|url=%s|req=%s|ret=%s|time=%d", url, req, r.Ret, emicros-smicros)
	return r, err
}
