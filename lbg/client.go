package lbg

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/bytedance/sonic"
)

type Client struct {
	conf   *ClientConfigure
	client *http.Client

	email  string
	userID string
	token  string
}

type ClientConfigure struct {
	Email    string
	Password string

	// Endpoint defaults to https://bohrium.dp.tech
	Endpoint string

	// Retry defaults to 0
	Retry int

	// Client defaults to http.DefaultClient
	Client *http.Client
}

func NewClient(conf *ClientConfigure) *Client {
	// Default values
	if conf.Endpoint == "" {
		conf.Endpoint = "https://bohrium.dp.tech"
	}
	if conf.Client == nil {
		conf.Client = http.DefaultClient
	}
	return &Client{
		conf:   conf,
		client: conf.Client,
		email:  conf.Email,
	}
}

func (c *Client) doRequest(
	method string,
	endpoint string,
	data []byte,
	headers http.Header,
	params url.Values,
) (
	statusCode int,
	response []byte,
	err error,
) {
	var uri *url.URL
	uri, err = url.Parse(
		strings.Join([]string{
			path.Join(c.conf.Endpoint, endpoint),
			"?",
			params.Encode(),
		}, ""),
	)
	if err != nil {
		return 0, nil, err
	}
	var header http.Header
	if headers == nil {
		header = make(http.Header)
	} else {
		header = headers.Clone()
	}
	if c.token != "" {
		header.Add("Authorization", fmt.Sprintf("jwt %s", c.token))
	}
	// TODO: version; now pseudo lbg utility 1.2.18
	header.Add("bohr-client", "utility:1.2.18")
	hc := c.client

	for trial := 0; trial < c.conf.Retry; trial++ {
		statusCode, response, err = func() (
			statusCode int,
			response []byte,
			err error,
		) {
			var bodyBuf io.ReadCloser
			if data != nil {
				bodyBuf = io.NopCloser(bytes.NewReader(data))
			}
			req := &http.Request{
				Header:        header,
				Method:        method,
				URL:           uri,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				ContentLength: int64(len(data)),
				Host:          uri.Host,
				Body:          bodyBuf,
			}
			req.GetBody = func() (io.ReadCloser, error) { return bodyBuf, nil }
			resp, err := hc.Do(req)
			if err != nil {
				// TODO: better error handling
				return resp.StatusCode, nil, err
			}

			defer resp.Body.Close()
			var buf bytes.Buffer
			_, err = buf.ReadFrom(resp.Body)
			if err != nil {
				return resp.StatusCode, nil, err
			}
			// Parse response json
			node, err := sonic.Get(buf.Bytes(), "code")
			if err != nil {
				return resp.StatusCode, nil, err
			}
			code, err := node.String()
			if err != nil {
				return resp.StatusCode, nil, err
			}
			if code != "0" && code != "0000" {
				msg, err := sonic.Get(buf.Bytes(), "message")
				if err != nil {
					m, _ := msg.String()
					return resp.StatusCode, nil, fmt.Errorf("%s", m)
				}
				msg, err = sonic.Get(buf.Bytes(), "error")
				if err != nil {
					m, _ := msg.String()
					return resp.StatusCode, nil, fmt.Errorf("%s", m)
				}
				return resp.StatusCode, nil, fmt.Errorf("non zero response code %v", code)
			}
			dataRaw, err := sonic.Get(buf.Bytes(), "data")
			var respReal []byte
			if err == nil {
				respStr, _ := dataRaw.Raw()
				respReal = []byte(respStr)
			}
			return resp.StatusCode, respReal, nil
		}()
		if err == nil {
			break
		}
	}
	return statusCode, response, err
}

func (c *Client) Login() error {
	if c.conf.Email == "" {
		return fmt.Errorf("email not set")
	}
	if c.conf.Password == "" {
		return fmt.Errorf("password not set")
	}
	c.email = c.conf.Email
	_, resp, err := c.doRequest(
		"POST",
		"/account/login",
		[]byte(fmt.Sprintf(`{"email":"%s","password":"%s"}`, c.email, c.conf.Password)),
		nil,
		nil,
	)
	if err != nil {
		return err
	}
	log.Println(string(resp))
	node, err := sonic.Get(resp, "token")
	if err != nil {
		return err
	}
	token, err := node.String()
	if err != nil {
		return err
	}
	c.token = token
	return nil
}
