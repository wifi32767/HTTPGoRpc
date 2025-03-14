package gorpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/wifi32767/GoRpc/codec"
)

// 这两个结构体用于注册中心的注册
type ServiceInfo struct {
	Name    string
	Addr    string
	Timeout time.Duration
}

type Service struct {
	Info         ServiceInfo
	LastPingTime time.Time
}

type Server struct {
	Name             string
	Addr             string
	Port             string
	HeartBeatTimeout time.Duration
	ServiceMap       sync.Map
	srv              *http.Server
	cli              *http.Client
}

// 传入一个自定义的struct对象
// 该结构体的所有public方法都会被注册
// 这些方法必须是形如func(req, resp any) error的形式
// 其中req是请求的值，resp是一个用于获取结果的指针
func NewServer(name, port string, server any, heartbeattimeout time.Duration) (*Server, error) {
	addr := getLocalIP()
	if addr == "" {
		err := fmt.Errorf("cannot get local ip")
		slog.Error(err.Error())
		return nil, err
	}
	srv := &Server{
		Name:             name,
		Addr:             getLocalIP(),
		Port:             port,
		HeartBeatTimeout: heartbeattimeout,
		ServiceMap:       sync.Map{},
		srv: &http.Server{
			Addr: port,
		},
		cli: &http.Client{},
	}
	// 注册所有的public方法
	t := reflect.TypeOf(server)
	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		srv.ServiceMap.Store(method.Name, &Method{
			method:   method,
			ArgType:  method.Type.In(1),
			RetType:  method.Type.In(2),
			Receiver: reflect.ValueOf(server),
		})
	}
	slog.Info(fmt.Sprintf("rpc server: service %s registerd", name))
	http.HandleFunc("/call", srv.handler)
	return srv, nil
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	// 判断是否是一个调用
	if r.Header.Get("X-Type") != TypeCall {
		slog.Debug(r.Header.Get("X-Type"))
		slog.Error("rpc server: wrong message type")
		s.sendErr(w, fmt.Errorf("rpc server: wrong message type"), http.StatusBadRequest)
		return
	}

	// 解析头部
	header, err := s.parseHeader(r)
	if err != nil {
		slog.Error("rpc server: parse header failed", "err", err)
		s.sendErr(w, err, http.StatusBadRequest)
		return
	}

	// 验证请求
	if err := s.validateReq(header); err != nil {
		s.sendErr(w, err, http.StatusBadRequest)
		return
	}

	// 创建编解码器
	cc := codec.NewCodec(header.Option.CodecType)
	if cc == nil {
		slog.Error("rpc server: unsupported codec type", "codec type", header.Option.CodecType)
		s.sendErr(w, fmt.Errorf("rpc server: unsupported codec type %s", header.Option.CodecType), http.StatusBadRequest)
		return
	}

	// 处理请求
	if err := s.processReq(w, cc, header, r.Body); err != nil {
		s.sendErr(w, err, http.StatusInternalServerError)
		return
	}

}

func (s *Server) parseHeader(r *http.Request) (*Header, error) {
	h := r.Header.Get("X-Header")
	if h == "" {
		return nil, fmt.Errorf("rpc server: header is empty")
	}
	var header Header
	if err := json.Unmarshal([]byte(h), &header); err != nil {
		return nil, err
	}
	return &header, nil
}

func (s *Server) validateReq(header *Header) error {
	// 验证magic number
	if header.Option.MagicNumber != MagicNumber {
		slog.Error("rpc server: invalid magic number", "magic number", header.Option.MagicNumber)
		return fmt.Errorf("rpc server: invalid magic number %d", header.Option.MagicNumber)
	}

	// 确认服务名正确
	if s.Name != header.Service {
		slog.Error("rpc server: service name mismatch", s.Name, header.Service)
		return fmt.Errorf("rpc server: service name mismatch %s", header.Service)
	}

	// 确认这个方法存在
	_, ok := s.ServiceMap.Load(header.Method)
	if !ok {
		slog.Error("rpc server: method not found", "method", header.Method)
		return fmt.Errorf("rpc server: method not found %s", header.Method)
	}

	return nil
}

func (s *Server) processReq(w http.ResponseWriter, cc codec.Codec, header *Header, body io.Reader) error {
	m, ok := s.ServiceMap.Load(header.Method)
	if !ok {
		slog.Error("rpc server: method not found", "method", header.Method)
		return fmt.Errorf("rpc server: method not found %s", header.Method)
	}
	// 获取body
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		slog.Error("rpc server: read body failed", "err", err)
		return err
	}
	// 解码body
	method := m.(*Method)
	req := method.newArgv().Interface()
	if method.newArgv().Type().Kind() != reflect.Ptr {
		req = method.newArgv().Addr().Interface()
	}
	if err := cc.Decode(bodyBytes, req); err != nil {
		slog.Error("rpc server: decode body failed", "err", err)
		return err
	}
	slog.Debug("rpc server: request", req)
	// 调用方法
	resp, err := s.call(method, req)
	if err != nil {
		slog.Error("rpc server: call method failed", "err", err)
		s.sendErr(w, err, http.StatusInternalServerError)
		return err
	}
	// 编码结果
	msg, err := cc.Encode(resp)
	if err != nil {
		slog.Error("rpc server: encode response failed", "err", err)
		s.sendErr(w, err, http.StatusInternalServerError)
		return err
	}
	// 发送结果
	s.sendResp(w, msg)
	return nil
}

func (s *Server) sendResp(w http.ResponseWriter, msg []byte) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(msg)
	if err != nil {
		slog.Error("rpc server: write response failed", "err", err)
	}
}

func (s *Server) sendErr(w http.ResponseWriter, err error, statusCode int) {
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(err.Error()))
}

func (s *Server) call(method *Method, req any) (any, error) {
	// 校验参数类型
	if reflect.TypeOf(req) != method.ArgType {
		slog.Error("rpc server: request type mismatch", "type of req", reflect.TypeOf(req), "type of arg", method.ArgType)
		return nil, fmt.Errorf("rpc server: request type mismatch %s", reflect.TypeOf(req))
	}
	f := method.method.Func
	ret := method.newRetv()
	// 实际的调用
	errRet := f.Call([]reflect.Value{method.Receiver, reflect.ValueOf(req), ret})
	if len(errRet) == 0 {
		return nil, fmt.Errorf("rpc server: no return value")
	}
	if errRet[0].Interface() == nil {
		return ret.Interface(), nil
	}
	return ret.Interface(), errRet[0].Interface().(error)
}

func (s *Server) Run() error {
	slog.Info("rpc server: Running")
	return s.srv.ListenAndServe()
}

func (s *Server) RunWithRegistry(registryAddr string) error {
	s.register(registryAddr, s.HeartBeatTimeout)
	go s.heartBeat(registryAddr, s.HeartBeatTimeout)
	return s.Run()
}

func (s *Server) register(registryAddr string, timeout time.Duration) {
	service := ServiceInfo{
		Name:    s.Name,
		Addr:    s.Addr + s.Port,
		Timeout: timeout,
	}
	body, err := json.Marshal(service)
	if err != nil {
		slog.Error("rpc server: marshal service info failed", "err", err)
		return
	}
	req, err := http.NewRequest("POST", registryAddr+"/register", bytes.NewBuffer(body))
	if err != nil {
		slog.Error("rpc server: new request failed", "err", err)
		return
	}
	req.Header.Set("X-Type", TypeRegister)
	resp, err := s.cli.Do(req)
	if err != nil {
		slog.Error("rpc server: send request failed", "err", err)
		return
	}
	defer resp.Body.Close()
}

func (s *Server) heartBeat(registryAddr string, timeout time.Duration) {
	info := ServiceInfo{
		Name:    s.Name,
		Addr:    s.Addr + s.Port,
		Timeout: s.HeartBeatTimeout,
	}
	b, err := json.Marshal(info)
	if err != nil {
		slog.Error("rpc server: marshal failed", "err", err)
		return
	}

	for {
		time.Sleep(timeout)
		req, err := http.NewRequest("POST", registryAddr+"/heartbeat", bytes.NewBuffer(b))
		if err != nil {
			slog.Error("rpc server: new request failed", "err", err)
			return
		}
		req.Header.Set("X-Type", TypePing)

		_, err = s.cli.Do(req)
		if err != nil {
			slog.Error("rpc server: send request failed", "err", err)
			continue
		}
	}
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalf("Failed to get interface addresses: %v", err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return ""
}
