package registry

import (
	gorpc "github.com/wifi32767/HTTPGoRpc"
)

// 通过这个接口，可以实现不同的负载均衡算法
// 暂时是不支持带权的算法
// 注意这个接口要自行保证线程安全，外部不为其加锁
type LoadBalance interface {
	Register(info gorpc.ServiceInfo)
	HeartBeat(name, addr string)
	Get(name string, timeoutFactor float64) (string, error)
}

type Constructor func() LoadBalance
type Type string
type ConstructorMap map[Type]Constructor

const (
	TypeRoundRobin Type = "round_robin"
)

var LoadBalanceMap = map[Type]func() LoadBalance{
	TypeRoundRobin: NewRoundRobin,
}

func RegisterLoadBalance(t Type, f Constructor) {
	LoadBalanceMap[t] = f
}

func NewLoadBalance(t Type) LoadBalance {
	if f, ok := LoadBalanceMap[t]; ok {
		return f()
	}
	return nil
}
