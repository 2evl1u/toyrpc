package toyrpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	. "toyrpc/log"
)

const (
	DefaultRegisterPath    = "/default_registry"
	DefaultRegistryPort    = ":9999"
	DefaultTimeoutInterval = 2 * time.Minute
)

type Registry struct {
	port     string
	services map[string]*struct {
		addresses map[string]time.Time
		timeout   time.Duration // 此服务需要检查是否存活的间隔（不同服务对于存活检查间隔要求可能是不同的）
	}
	mu *sync.Mutex
}

// svcUpdateMapping 客户端发送心跳，注册服务用用的接口映射
type svcUpdateMapping struct {
	ServiceName string `json:"serviceName"`
	ServiceAddr string `json:"serviceAddr"`
}

// NewRegistry 新建一个注册中心服务端，默认端口是:9999
func NewRegistry(opts ...RegistryOpt) *Registry {
	r := &Registry{
		port: DefaultRegistryPort,
		services: make(map[string]*struct {
			addresses map[string]time.Time
			timeout   time.Duration
		}),
		mu: new(sync.Mutex),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	// GET方法用于获取对应服务的实例地址
	case http.MethodGet:
		serviceName := req.URL.Query().Get("serviceName")
		if serviceName == "" {
			w.WriteHeader(http.StatusBadRequest)
		}
		aliveServices := r.getAliveServices(serviceName)
		if err := json.NewEncoder(w).Encode(aliveServices); err != nil {
			ErrorLogger.Printf("Encode alive services fail:", err)
		} else {
			CommonLogger.Printf("Send service [%s]: %s to %s\n", serviceName, aliveServices, req.RemoteAddr)
		}
	// POST方法用于客户端发送心跳和注册服务
	case http.MethodPost:
		var b []byte
		b, err := io.ReadAll(req.Body)
		if err != nil {
			ErrorLogger.Printf("Read body fail: ", err)
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write([]byte(fmt.Sprintf("read body fail: %s\n", err)))
			if err != nil {
				ErrorLogger.Printf("Write fail: ", err)
			}
		}
		var res = new(svcUpdateMapping)
		if err = json.Unmarshal(b, res); err != nil {
			ErrorLogger.Printf("Unmarshal body fail: ", err)
			_, _ = w.Write([]byte(fmt.Sprintf("unmarshal body fail: %s\n", err)))
		}
		// 获取请求的ip地址
		lastIndex := strings.LastIndex(req.RemoteAddr, ":")
		reqIP := req.RemoteAddr[:lastIndex]
		res.ServiceAddr = reqIP + res.ServiceAddr
		// 更新对应服务实例的最后更新时间
		si, ok := r.services[res.ServiceName]
		// 服务实例信息存在
		if ok {
			// 注册/心跳请求的地址是否存在
			_, ok := si.addresses[res.ServiceAddr]
			// 不存在则加入，存在则更新
			si.addresses[res.ServiceAddr] = time.Now()
			if ok {
				CommonLogger.Printf("Get heartbeat from [%s %s]\n", res.ServiceName, res.ServiceAddr)
			} else {
				CommonLogger.Printf("Add new addr %s to service %s\n", res.ServiceAddr, res.ServiceName)
			}
		} else {
			// 该服务害不存在，第一次注册，则新建
			r.services[res.ServiceName] = &struct {
				addresses map[string]time.Time
				timeout   time.Duration
			}{
				addresses: make(map[string]time.Time),
				timeout:   DefaultTimeoutInterval,
			}
			r.services[res.ServiceName].addresses[res.ServiceAddr] = time.Now()
			CommonLogger.Printf("Register service [%s %s]\n", res.ServiceName, res.ServiceAddr)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Registry) Start() {
	http.Handle(DefaultRegisterPath, r)
	CommonLogger.Printf("Registry successfully starting at: %s\n", r.port)
	if err := http.ListenAndServe(r.port, nil); err != nil {
		ErrorLogger.Println(err)
	}
}

func (r *Registry) getAliveServices(serviceName string) []string {
	svcItem, ok := r.services[serviceName]
	if !ok {
		return nil
	}
	var aliveServices []string
	for addr, t := range svcItem.addresses {
		if t.Add(svcItem.timeout).After(time.Now()) {
			aliveServices = append(aliveServices, addr)
		} else {
			r.mu.Lock()
			delete(svcItem.addresses, addr)
			r.mu.Unlock()
		}
	}
	return aliveServices
}

type RegistryOpt func(registry *Registry)

func WithPort(port string) RegistryOpt {
	return func(r *Registry) {
		r.port = port
	}
}
