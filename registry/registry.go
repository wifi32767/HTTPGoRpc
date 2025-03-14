package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	gorpc "github.com/wifi32767/HTTPGoRpc"
)

// 这个设置比较原始，只有这一个项目
type Options struct {
	TimeoutFactor float64
}

type Registry struct {
	mtx     sync.Mutex
	Service map[string]map[string]*gorpc.Service
	Option  Options
	srv     *http.Server
}

func NewRegistry(port string, opt Options) *Registry {
	srv := &Registry{
		mtx: sync.Mutex{},
		srv: &http.Server{
			Addr: port,
		},
		Service: make(map[string]map[string]*gorpc.Service),
		Option:  opt,
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
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, ok := s.Service[info.Name]; !ok {
		s.Service[info.Name] = make(map[string]*gorpc.Service)
	}
	s.Service[info.Name][info.Addr] = &gorpc.Service{
		Info:         info,
		LastPingTime: time.Now(),
	}
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
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, ok := s.Service[methodName]; !ok {
		s.sendErr(w, fmt.Errorf("registry: service not found"), http.StatusNotFound)
		return
	}
	// 返回服务信息
	toDelete := []string{}
	flag := false
	for addr, service := range s.Service[methodName] {
		if service.LastPingTime.Before(time.Now().Add(-time.Duration(s.Option.TimeoutFactor) * service.Info.Timeout)) {
			toDelete = append(toDelete, addr)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(addr))
			flag = true
			break
		}
	}
	if !flag {
		s.sendErr(w, fmt.Errorf("registry: service not found"), http.StatusNotFound)
	}
	for _, addr := range toDelete {
		slog.Debug("delete: ", "addr", addr)
		delete(s.Service[methodName], addr)
	}
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
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if _, ok := s.Service[info.Name]; !ok {
		slog.Error("registry heartbeat: service not found")
		return
	}
	if _, ok := s.Service[info.Name][info.Addr]; !ok {
		slog.Error("registry heartbeat: service not found")
		return
	}
	s.Service[info.Name][info.Addr].LastPingTime = time.Now()
	w.WriteHeader(http.StatusOK)
}

func (s *Registry) sendErr(w http.ResponseWriter, err error, statusCode int) {
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(err.Error()))
}
