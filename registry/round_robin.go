package registry

import (
	"fmt"
	"sync"
	"time"

	gorpc "github.com/wifi32767/HTTPGoRpc"
)

type ServiceInfo struct {
	Name         string
	Addr         string
	LastPingTime time.Time
	Timeout      time.Duration
}

// 这个设计使用链表维护
type LinkedListNode struct {
	Pre  *LinkedListNode
	Next *LinkedListNode
	Body *ServiceInfo
}

type LinkedList struct {
	Cur  *LinkedListNode
	Size int
}

func NewLinkedList() *LinkedList {
	return &LinkedList{
		Cur:  nil,
		Size: 0,
	}
}

func (l *LinkedList) Add(name, addr string, timeout time.Duration) *ServiceInfo {
	node := &LinkedListNode{
		Body: &ServiceInfo{
			Name:         name,
			Addr:         addr,
			LastPingTime: time.Now(),
			Timeout:      timeout,
		},
	}
	if l.Size == 0 {
		l.Cur = node
		l.Cur.Next = l.Cur
		l.Cur.Pre = l.Cur
	} else {
		node.Next = l.Cur.Next
		node.Pre = l.Cur
		l.Cur.Next.Pre = node
		l.Cur.Next = node
	}
	l.Size++
	return node.Body
}

func (l *LinkedList) RemoveCur() {
	if l.Size == 0 {
		return
	}
	if l.Size == 1 {
		l.Cur = nil
		l.Size--
		return
	}
	l.Cur.Pre.Next = l.Cur.Next
	l.Cur.Next.Pre = l.Cur.Pre
	l.Cur = l.Cur.Next
	l.Size--
}

func (l *LinkedList) GetCur() *ServiceInfo {
	if l.Size == 0 {
		return nil
	}
	return l.Cur.Body
}

func (l *LinkedList) Next() {
	l.Cur = l.Cur.Next
}

type RoundRobin struct {
	// 服务列表，从中顺序选择一个可用的服务
	// 服务名 -> 服务列表
	ServiceMap map[string]*LinkedList
	// 服务信息，从这里的映射更新，比链表快
	// 服务器地址 -> 服务信息
	Info  map[string]*ServiceInfo
	mutex sync.Mutex
}

func NewRoundRobin() LoadBalance {
	return &RoundRobin{
		ServiceMap: make(map[string]*LinkedList),
		Info:       make(map[string]*ServiceInfo),
	}
}

func (r *RoundRobin) Register(info gorpc.ServiceInfo) {
	name := info.Name
	addr := info.Addr
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.ServiceMap[name]; !ok {
		r.ServiceMap[name] = NewLinkedList()
	}
	i := r.ServiceMap[name].Add(name, addr, info.Timeout)
	r.Info[addr] = i
}

func (r *RoundRobin) HeartBeat(name, addr string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.Info[addr]; !ok {
		return
	}
	r.Info[addr].LastPingTime = time.Now()
}

func (r *RoundRobin) Get(name string, factor float64) (string, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.ServiceMap[name]; !ok {
		return "", fmt.Errorf("service %s not found", name)
	}
	for r.ServiceMap[name].Size > 0 {
		cur := r.ServiceMap[name].GetCur()
		if cur == nil {
			return "", fmt.Errorf("service %s not found", name)
		}
		if cur.LastPingTime.Add(cur.Timeout * time.Duration(factor)).Before(time.Now()) {
			r.ServiceMap[name].RemoveCur()
			delete(r.Info, cur.Addr)
			continue
		}
		break
	}
	if r.ServiceMap[name].Size == 0 {
		return "", fmt.Errorf("service %s not found", name)
	}
	defer r.ServiceMap[name].Next()
	return r.ServiceMap[name].GetCur().Addr, nil
}
