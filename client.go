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
		slog.Error("rpc client: parse option failed", err)
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
	// 创建请求头
	h := Header{
		Service: service,
		Method:  method,
		Option:  c.Opt,
	}
	header, err := json.Marshal(h)
	if err != nil {
		slog.Error("rpc client: marshal header failed", err)
		return err
	}
	// 创建请求体
	body, err := c.cc.Encode(arg)
	if err != nil {
		slog.Error("rpc client: encode failed", err)
		return err
	}
	// 发送请求
	req, err := http.NewRequest("POST", c.TargetAddr+"/call", bytes.NewBuffer(body))
	if err != nil {
		slog.Error("rpc client: new request failed", err)
		return err
	}
	req.Header.Set("X-Type", TypeCall)
	req.Header.Set("X-Header", string(header))
	resp, err := c.cli.Do(req)
	if err != nil {
		slog.Error("rpc client: send request failed", err)
		return err
	}
	defer resp.Body.Close()
	// 解析响应
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("rpc client: read response failed", err)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("[%d] %s", resp.StatusCode, res)
	}
	err = c.cc.Decode(res, ret)
	if err != nil {
		slog.Error("rpc client: decode response failed", err)
		return err
	}
	return nil
}
