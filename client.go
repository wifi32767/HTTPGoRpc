package gorpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/wifi32767/GoRpc/codec"
)

type Client struct {
	TargetAddr string
	Opt        Options
	cc         codec.Codec
	cli        *http.Client
}

func NewClient(addr string, opts ...*Options) *Client {
	// 解析设置
	opt, err := parseOptions(opts...)
	if err != nil {
		slog.Error("rpc client: parse option failed", "err", err)
		return nil
	}
	// 生成编解码器
	cc := codec.NewCodec(opt.CodecType)
	if cc == nil {
		slog.Error("rpc client: new codec failed")
		return nil
	}

	return &Client{
		TargetAddr: addr,
		Opt:        *opt,
		cc:         cc,
		cli:        &http.Client{},
	}
}

func parseOptions(opts ...*Options) (*Options, error) {
	var opt *Options
	if len(opts) == 0 || opts[0] == nil {
		opt = DefaultOptions
	} else if len(opts) > 1 {
		return nil, fmt.Errorf("number of options is more than 1")
	} else {
		opt = opts[0]
	}

	opt.MagicNumber = MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = codec.TypeGob
	}
	return opt, nil
}

func (c *Client) Call(service, method string, arg any, ret any) error {
	if c.Opt.UseRegistry {
		// 从注册中心获取服务地址
		addr, err := c.getAddr(service)
		if err != nil {
			slog.Error("rpc client: get addr failed", "err", err)
			return err
		}
		return c.call(addr, service, method, arg, ret)
	}
	return c.call(c.TargetAddr, service, method, arg, ret)
}

func (c *Client) getAddr(service string) (string, error) {
	req, err := http.NewRequest("POST", c.TargetAddr+"/get", bytes.NewBufferString(service))
	if err != nil {
		slog.Error("rpc client: new request failed", "err", err)
		return "", err
	}
	req.Header.Set("X-Type", TypeAsk)
	resp, err := c.cli.Do(req)
	if err != nil {
		slog.Error("rpc client: send request failed", "err", err)
		return "", err
	}
	defer resp.Body.Close()
	addr, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("rpc client: read response failed", "err", err)
		return "", err
	}
	return string(addr), nil
}

func (c *Client) call(addr, service, method string, arg any, ret any) error {
	// 创建请求头
	h := Header{
		Service: service,
		Method:  method,
		Option:  c.Opt,
	}
	header, err := json.Marshal(h)
	if err != nil {
		slog.Error("rpc client: marshal header failed", "err", err)
		return err
	}
	// 创建请求体
	body, err := c.cc.Encode(arg)
	if err != nil {
		slog.Error("rpc client: encode failed", "err", err)
		return err
	}
	// 发送请求
	resp, err := c.sendReq(TypeCall, addr, header, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 解析响应
	err = c.parseResp(resp, ret)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) sendReq(typ, addr string, header, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", "http://"+addr+"/call", bytes.NewBuffer(body))
	if err != nil {
		slog.Error("rpc client: new request failed", "err", err)
		return nil, err
	}
	req.Header.Set("X-Type", typ)
	req.Header.Set("X-Header", string(header))
	resp, err := c.cli.Do(req)
	if err != nil {
		slog.Error("rpc client: send request failed", "err", err)
		return nil, err
	}
	return resp, nil
}

func (c *Client) parseResp(resp *http.Response, ret any) error {
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("rpc client: read response failed", "err", err)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("[%d] %s", resp.StatusCode, res)
	}
	err = c.cc.Decode(res, ret)
	if err != nil {
		slog.Error("rpc client: decode response failed", "err", err)
		return err
	}
	return nil
}
