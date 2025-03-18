package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	gorpc "github.com/wifi32767/HTTPGoRpc"
)

// 这个设置比较原始，只有这两个项目
type Options struct {
	TimeoutFactor float64
	LoadBalance   Type
}

var DefaultOptions = &Options{
	TimeoutFactor: 3,
	LoadBalance:   TypeRoundRobin,
}

type Registry struct {
	LoadBalance LoadBalance
	Option      *Options
	srv         *http.Server
}

// NewRegistry 创建一个新的 Registry 实例，使用指定的端口和选项。
// 它解析提供的选项，初始化负载均衡器，并设置 HTTP 处理程序
// 用于服务注册、检索和心跳。
//
// 参数:
//   - port: 注册中心服务器将监听的端口。
//   - opts: 一个可变参数列表，包含指向配置注册表的 Options 的指针。
//
// 返回值:
//   - *Registry: 指向初始化的 Registry 实例的指针，如果发生错误则返回 nil。
func NewRegistry(port string, opts ...*Options) *Registry {
	opt, err := parseOptions(opts...)
	if err != nil {
		slog.Error("registry: parse option failed", "err", err)
		return nil
	}
	lb := NewLoadBalance(opt.LoadBalance)
	if lb == nil {
		slog.Error("registry: load balance not found")
		return nil
	}
	srv := &Registry{
		srv: &http.Server{
			Addr: port,
		},
		LoadBalance: lb,
		Option:      opt,
	}
	http.HandleFunc("/register", srv.register)
	http.HandleFunc("/get", srv.get)
	http.HandleFunc("/heartbeat", srv.heartBeat)
	return srv
}

// parseOptions 解析可选参数并返回一个 Options 指针。
// 如果没有提供参数或第一个参数为 nil，则返回默认选项 DefaultOptions。
// 如果提供的参数多于一个，则返回错误。
// 参数:
//
//	opts: 可变参数列表，包含一个或零个 Options 指针。
//
// 返回值:
//
//	*Options: 解析后的 Options 指针。
//	error: 如果提供的参数多于一个，则返回错误信息。
func parseOptions(opts ...*Options) (*Options, error) {
	var opt *Options
	if len(opts) == 0 || opts[0] == nil {
		opt = DefaultOptions
	} else if len(opts) > 1 {
		return nil, fmt.Errorf("number of options is more than 1")
	} else {
		opt = opts[0]
	}
	return opt, nil
}

// Run 启动注册表服务并监听传入的请求。
// 如果服务启动成功，则返回 nil，否则返回错误。
func (s *Registry) Run() error {
	slog.Info("registry: Running")
	return s.srv.ListenAndServe()
}

// register 处理服务注册请求。
// 它首先检查请求头中的 "X-Type" 是否为 gorpc.TypeRegister，以确定是否为注册请求。
// 然后读取请求体中的服务信息，并将其解析为 gorpc.ServiceInfo 结构。
// 如果解析成功，则调用 LoadBalance 的 Register 方法注册服务。
// 最后返回 HTTP 状态码 200 表示成功。
func (s *Registry) register(w http.ResponseWriter, r *http.Request) {
	// 判断是否是一个注册
	if r.Header.Get("X-Type") != gorpc.TypeRegister {
		slog.Error("registry: wrong message type")
		s.sendErr(w, fmt.Errorf("registry: wrong message type"), http.StatusBadRequest)
		return
	}
	// 获取服务信息
	b, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("registry: read body failed", "err", err)
		s.sendErr(w, err, http.StatusBadRequest)
		return
	}
	info := gorpc.ServiceInfo{}
	if err = json.Unmarshal(b, &info); err != nil {
		slog.Error("registry: parse body failed", "err", err)
		s.sendErr(w, err, http.StatusBadRequest)
		return
	}
	// 注册服务
	s.LoadBalance.Register(info)
	w.WriteHeader(http.StatusOK)
}

// get 处理客户端请求，根据请求体中提供的方法名称检索服务地址。
// 它执行以下步骤：
// 1. 检查请求头 "X-Type" 是否等于 gorpc.TypeAsk。如果不是，则记录错误并发送 BadRequest 响应。
// 2. 读取请求体以获取方法名称。
// 3. 使用 LoadBalance 组件获取给定方法名称的服务地址。
func (s *Registry) get(w http.ResponseWriter, r *http.Request) {
	// 判断是否是一个调用
	if r.Header.Get("X-Type") != gorpc.TypeAsk {
		slog.Error("registry: wrong message type")
		s.sendErr(w, fmt.Errorf("registry: wrong message type"), http.StatusBadRequest)
		return
	}
	// 获取服务信息
	b, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("registry: read body failed", "err", err)
		s.sendErr(w, err, http.StatusBadRequest)
		return
	}
	methodName := string(b)
	// 获取服务
	addr, err := s.LoadBalance.Get(methodName, s.Option.TimeoutFactor)
	if err != nil {
		slog.Error("registry: get service failed", "err", err)
		s.sendErr(w, err, http.StatusNotFound)
		return
	}
	// 返回服务信息
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(addr))
}

// heartBeat 处理心跳请求。
// 它首先检查请求头中的 "X-Type" 是否为 gorpc.TypePing，以确定是否为心跳消息。
// 然后读取请求体并将其反序列化为 gorpc.ServiceInfo 结构。
// 最后，它更新服务的心跳时间并返回 HTTP 200 状态码。
func (s *Registry) heartBeat(w http.ResponseWriter, r *http.Request) {
	// 判断是否是一个心跳
	if r.Header.Get("X-Type") != gorpc.TypePing {
		slog.Error("registry heartbeat: wrong message type")
		return
	}
	// 获取信息
	b, err := io.ReadAll(r.Body)
	slog.Debug(string(b))
	if err != nil {
		slog.Error("registry heartbeat: read body failed", "err", err)
		return
	}
	info := gorpc.ServiceInfo{}
	err = json.Unmarshal(b, &info)
	if err != nil {
		slog.Error("registry heartbeat: body unmarshal failed", "err", err)
	}
	// 更新心跳时间
	s.LoadBalance.HeartBeat(info.Name, info.Addr)
	w.WriteHeader(http.StatusOK)
}

// sendErr 向 HTTP 响应写入错误信息和状态码。
// 参数:
//
//	w: HTTP 响应写入器。
//	err: 要写入的错误信息。
//	statusCode: HTTP 状态码。
func (s *Registry) sendErr(w http.ResponseWriter, err error, statusCode int) {
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(err.Error()))
}
