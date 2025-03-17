使用Go语言和HTTP协议实现了一个简单的RPC框架  
支持服务注册、超时处理、负载均衡  

### 编解码器
提供编解码功能，进行结构体和字节流之间的转换  
```go
// codec/codec.go

type Codec interface {
	Encode(msg any) ([]byte, error)
	EncodeString(msg any) (string, error)
	// 解码的 msg 必须是指针类型
	Decode(data []byte, msg any) error
	DecodeString(data string, msg any) error
}

type Type string

// 注册自定义的编解码器
func RegisterCodec(t Type, f CodecConstructor)

// 获取已经注册的编解码器
func NewCodec(t Type) Codec
```
附带两个实现，分别基于gob和json

### 服务端
对于传入的结构体，注册它所有的public方法，使用post方法调用对应的接口可以调用该方法  
可以注册到注册中心  

```go

// 创建一个服务器，将server对象的public方法注册在服务器上
func NewServer(name, port string, server any, heartbeatTimeout time.Duration) (*Server, error)

// 运行服务器，使其监听预设的端口
func (s *Server) Run() error

// 运行的同时连接对应的注册中心
func (s *Server) RunWithRegistry(registryAddr string) error

// 使用例
type T struct{}

type Req struct {
	Name string
	Id   int
}

type Resp struct {
	Res string
}

func (t *T) Fun1(req *Req, resp *Resp) error {
	resp.Res = fmt.Sprintf("%s: %d", req.Name, req.Id)
	return nil
}

func main() {
	srv, err := gorpc.NewServer("T", ":2222", &T{}, time.Second)
	if err != nil {
		fmt.Println(err)
	}
    // 执行Run之后，访问http://localhost:2222/call可以调用对应的函数
    // 消息格式：

    // Header：
    // X-Type 消息类型
    // X-Header json格式的头部

    // Body：
    // 编码后的请求体正文
	err = srv.RunWithRegistry("http://localhost:1111")
	if err != nil {
		fmt.Println(err)
	}
}
```

### 客户端
进行RPC调用的客户端

```go
type Options struct {
	MagicNumber int
	CodecType   codec.Type
	UseRegistry bool
}

// 创建一个新的客户端，addr是客户端要连接的地址
// 这个地址可以是注册中心的地址，也可以是服务端的地址
// 至于是哪一个，要在option中写明
func NewClient(addr string, opts ...*Options) *Client

// 进行一次RPC调用
// ctx用于http协议的超时控制
// arg传入请求值，ret传入接收返回值的指针
func (c *Client) Call(ctx context.Context, service, method string, arg any, ret any) error

// 异步RPC调用
// 返回的通道接收到数据的时候表明调用完成
func (c *Client) AsyncCall(ctx context.Context, service, method string, arg any, ret any) chan error 

// 使用例
func main() {
    cli := gorpc.NewClient("http://localhost:1111", &gorpc.Options{
        UseRegistry: true,
    })
    ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
    ch := cli.AsyncCall(ctx, "T", "Fun1", Req{"hello", 1}, &resp)
    err := <-ch
    if err != nil {
        fmt.Println(err)
    }
    fmt.Println(resp)
}
```

### 注册中心
注册中心，可以注册多个服务端，接收客户端的请求  
收到请求时，会通过负载均衡算法在对应服务名的多个服务中选择一个  
返回其地址，由客户端自行调用  
服务端定期发送心跳保活信号，确认存活  
太长时间不发送信号的服务端会被踢出  
```go
// 提供了负载均衡接口
// 负载均衡策略和踢出服务的策略完全通过实现这个接口决定
// 每个注册中心中会有一个这样的结构体，通过调用其方法来实现负载均衡
// 提供了一个基于环形链表的轮询算法实现
type LoadBalance interface {
	Register(info gorpc.ServiceInfo)
	HeartBeat(name, addr string)
	Get(name string, timeoutFactor float64) (string, error)
}

func NewRegistry(port string, opt Options) *Registry

func (s *Registry) Run() error

// 使用例
func main() {
	reg := registry.NewRegistry(":1111", &registry.Options{
		TimeoutFactor: 3,
		LoadBalance:   registry.TypeRoundRobin,
	})
	err := reg.Run()
	if err != nil {
		fmt.Println(err)
	}
}
```