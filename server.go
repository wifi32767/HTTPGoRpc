package gorpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"sync"

	"github.com/wifi32767/GoRpc/codec"
)

type Server struct {
	Name       string
	ServiceMap sync.Map
}

// 传入一个自定义的struct对象
// 该结构体的所有public方法都会被注册
// 这些方法必须是形如func(req, resp any) error的形式
// 其中req是请求的值，resp是一个用于获取结果的指针
func NewServer(name string, server any) *Server {
	srv := &Server{
		Name:       name,
		ServiceMap: sync.Map{},
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
	return srv
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	// 判断是否是一个调用
	if r.Header.Get("X-Type") != TypeCall {
		slog.Error("rpc server: wrong message type")
		s.sendErr(w, fmt.Errorf("rpc server: wrong message type"), http.StatusBadRequest)
		return
	}

	// 解析头部
	header, err := s.parseHeader(r)

	if err != nil {
		slog.Error("rpc server: parse header failed", err)
		s.sendErr(w, err, http.StatusBadRequest)
		return
	}
	// 验证magic number
	if header.Option.MagicNumber != MagicNumber {
		slog.Error("rpc server: invalid magic number", header.Option.MagicNumber)
		s.sendErr(w, fmt.Errorf("rpc server: invalid magic number %d", header.Option.MagicNumber), http.StatusBadRequest)
		return
	}
	// 创建编解码器
	cc := codec.NewCodec(header.Option.CodecType)
	if cc == nil {
		slog.Error("rpc server: unsupported codec type", header.Option.CodecType)
		s.sendErr(w, fmt.Errorf("rpc server: unsupported codec type %s", header.Option.CodecType), http.StatusBadRequest)
		return
	}
	// 确认服务名正确
	if s.Name != header.Service {
		slog.Error("rpc server: service name mismatch", s.Name, header.Service)
		s.sendErr(w, fmt.Errorf("rpc server: service name mismatch %s", header.Service), http.StatusBadRequest)
		return
	}
	// 确认这个方法存在
	m, ok := s.ServiceMap.Load(header.Method)
	if !ok {
		slog.Error("rpc server: method not found", header.Method)
		s.sendErr(w, fmt.Errorf("rpc server: method not found %s", header.Method), http.StatusBadRequest)
		return
	}
	// 获取body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("rpc server: read body failed", err)
		s.sendErr(w, err, http.StatusBadRequest)
		return
	}
	// 解码body
	method := m.(*Method)
	req := method.newArgv().Interface()
	if method.newArgv().Type().Kind() != reflect.Ptr {
		req = method.newArgv().Addr().Interface()
	}
	if err := cc.Decode(body, req); err != nil {
		slog.Error("rpc server: decode body failed", err)
		s.sendErr(w, err, http.StatusBadRequest)
	}
	slog.Debug("rpc server: request", req)
	// 调用方法
	resp, err := s.call(method, req)
	if err != nil {
		slog.Error("rpc server: call method failed", err)
		s.sendErr(w, err, http.StatusInternalServerError)
		return
	}
	// 编码结果
	msg, err := cc.Encode(resp)
	if err != nil {
		slog.Error("rpc server: encode response failed", err)
		s.sendErr(w, err, http.StatusInternalServerError)
		return
	}
	// 发送结果
	s.sendResp(w, msg)
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

func (s *Server) sendResp(w http.ResponseWriter, msg []byte) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(msg)
	if err != nil {
		slog.Error("rpc server: write response failed", err)
	}
}

func (s *Server) sendErr(w http.ResponseWriter, err error, statusCode int) {
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(err.Error()))
}

func (s *Server) call(method *Method, req any) (any, error) {
	// 校验参数类型
	if reflect.TypeOf(req) != method.ArgType {
		slog.Error("rpc server: request type mismatch", reflect.TypeOf(req), method.ArgType)
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

func (s *Server) Run(port string) error {
	return http.ListenAndServe(port, nil)
}
