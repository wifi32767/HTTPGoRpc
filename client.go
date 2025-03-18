package gorpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/wifi32767/HTTPGoRpc/codec"
)

type Client struct {
	// 目标地址，如果使用注册中心则为注册中心地址
	// 否则为服务端地址
	TargetAddr string
	Opt        Options
	cc         codec.Codec
	cli        *http.Client
}

// NewClient 创建一个新的 RPC 客户端实例
// 参数:
//   - addr: 服务端地址或注册中心地址，取决于设置中是否使用注册中心。
//   - opts: 一个可变参数列表，包含指向配置客户端的 Options 的指针。
//
// 返回值:
//   - *Client: 指向初始化的 Client 实例的指针，如果发生错误则返回 nil。
func NewClient(addr string, opts ...*Options) *Client {
	// 解析设置
	opt, err := parseOptions(opts...)
	if err != nil {
		slog.Error("rpc client: 解析选项失败", "err", err)
		return nil
	}
	// 创建编解码器
	cc := codec.NewCodec(opt.CodecType)
	if cc == nil {
		slog.Error("rpc client: 创建编解码器失败")
		return nil
	}

	return &Client{
		TargetAddr: addr,
		Opt:        *opt,
		cc:         cc,
		cli:        &http.Client{},
	}
}

// parseOptions 解析设置
// 参数:
//   - opts: 一个可变参数列表，包含指向配置客户端的 Options 的指针。
//
// 返回值:
//   - *Options: 指向初始化的 Options 实例的指针，如果发生错误则返回 nil。
//   - error: 如果发生错误，则返回错误信息。
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

// Call 同步调用 RPC 服务
// 参数:
//   - ctx: 上下文
//   - service: 服务名
//   - method: 方法名
//   - arg: 参数
//   - ret: 返回值指针
//
// 返回值:
//   - error: 如果发生错误，则返回错误信息。
func (c *Client) Call(ctx context.Context, service, method string, arg any, ret any) error {
	if c.Opt.UseRegistry {
		// 从注册中心获取服务地址
		addr, err := c.getAddr(service)
		if err != nil {
			slog.Error("rpc client: get addr failed", "err", err)
			return err
		}
		return c.call(ctx, addr, service, method, arg, ret)
	}
	return c.call(ctx, c.TargetAddr, service, method, arg, ret)
}

// getAddr 从注册中心获取服务地址
// 参数:
//   - service: 服务名
//
// 返回值:
//   - string: 服务地址
//   - error: 如果发生错误，则返回错误信息。
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

// call 真正的调用服务
// 参数:
//   - ctx: 上下文
//   - addr: 服务地址
//   - service: 服务名
//   - method: 方法名
//   - arg: 参数
//   - ret: 返回值指针
//
// 返回值:
//   - error: 如果发生错误，则返回错误信息。
func (c *Client) call(ctx context.Context, addr, service, method string, arg any, ret any) error {
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
	resp, err := c.sendReq(ctx, TypeCall, addr, header, body)
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

// sendReq 发送请求
// 参数:
//   - ctx: 上下文
//   - typ: 请求类型
//   - addr: 服务地址
//   - header: 请求头
//   - body: 请求体
//
// 返回值:
//   - *http.Response: HTTP 响应
//   - error: 如果发生错误，则返回错误信息。
func (c *Client) sendReq(ctx context.Context, typ, addr string, header, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", "http://"+addr+"/call", bytes.NewBuffer(body))
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

// parseResp 解析响应
// 将响应体解码为返回值
// 参数:
//   - resp: HTTP 响应
//   - ret: 返回值指针
//
// 返回值:
//   - error: 如果发生错误，则返回错误信息。
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

// AsyncCall 异步调用 RPC 服务
// 参数:
//   - ctx: 上下文
//   - service: 服务名
//   - method: 方法名
//   - arg: 参数
//   - ret: 返回值指针
//
// 返回值:
//   - chan error: 异步调用通道
func (c *Client) AsyncCall(ctx context.Context, service, method string, arg any, ret any) chan error {
	ch := make(chan error, 1)
	go func() {
		err := c.Call(ctx, service, method, arg, ret)
		ch <- err
	}()
	return ch
}
