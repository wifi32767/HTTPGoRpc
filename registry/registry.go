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

type Registry struct {
	LoadBalance LoadBalance
	Option      Options
	srv         *http.Server
}

func NewRegistry(port string, opt Options) *Registry {
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

func (s *Registry) Run() error {
	slog.Info("registry: Running")
	return s.srv.ListenAndServe()
}

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

func (s *Registry) sendErr(w http.ResponseWriter, err error, statusCode int) {
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(err.Error()))
}
