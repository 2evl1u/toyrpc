package toyrpc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRegisterPath    = "/default_registry"
	DefaultRegistryPort    = ":9999"
	DefaultTimeoutInterval = 2 * time.Minute
)

type ServiceItem struct {
	Addresses      map[string]time.Time
	updateInterval time.Duration // 此服务需要检查是否存活的间隔（不同服务对于存活检查间隔要求可能是不同的）
}

type Registry struct {
	port     string
	services map[string]*ServiceItem
	mu       *sync.Mutex
}

type SvcUpdateMapping struct {
	ServiceName string `json:"serviceName"`
	ServiceAddr string `json:"serviceAddr"`
}

func NewRegistry() *Registry {
	r := &Registry{
		port:     DefaultRegistryPort,
		services: make(map[string]*ServiceItem),
		mu:       new(sync.Mutex),
	}
	return r
}

func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		serviceName := req.URL.Query().Get("serviceName")
		if serviceName == "" {
			w.WriteHeader(http.StatusBadRequest)
		}
		aliveSvcs := r.getAliveServices(serviceName)
		if err := json.NewEncoder(w).Encode(aliveSvcs); err != nil {
			log.Println("encode alive services fail:", err)
		}
		log.Printf("send service [%s]: %s to %s\n", serviceName, aliveSvcs, req.RemoteAddr)
	case http.MethodPost:
		var b []byte
		b, err := io.ReadAll(req.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write([]byte(fmt.Sprintf("read body fail: %s\n", err)))
			if err != nil {
				log.Println(err)
			}
		}
		var res = new(SvcUpdateMapping)
		if err = json.Unmarshal(b, res); err != nil {
			log.Println(err)
			_, _ = w.Write([]byte(fmt.Sprintf("unmarshal body fail: %s\n", err)))
		}
		// 获取请求的ip地址
		lastIndex := strings.LastIndex(req.RemoteAddr, ":")
		ip := req.RemoteAddr[:lastIndex]
		res.ServiceAddr = ip + res.ServiceAddr
		// 更新对应服务实例的最后更新时间
		si, ok := r.services[res.ServiceName]
		if ok {
			_, ok := si.Addresses[res.ServiceAddr]
			si.Addresses[res.ServiceAddr] = time.Now()
			if ok {
				log.Printf("update service addr: %s[%s]\n", res.ServiceName, res.ServiceAddr)
			} else {
				log.Printf("register service addr: %s[%s]\n", res.ServiceName, res.ServiceAddr)
			}
		} else {
			r.services[res.ServiceName] = &ServiceItem{
				Addresses:      make(map[string]time.Time),
				updateInterval: DefaultTimeoutInterval,
			}
			r.services[res.ServiceName].Addresses[res.ServiceAddr] = time.Now()
			log.Printf("register service: %s address: %s\n", res.ServiceName, res.ServiceAddr)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Registry) Start() {
	http.Handle(DefaultRegisterPath, r)
	log.Printf("Registry start at: %s\n", r.port)
	if err := http.ListenAndServe(r.port, nil); err != nil {
		log.Println(err)
	}
}

func (r *Registry) getAliveServices(serviceName string) []string {
	svcItem, ok := r.services[serviceName]
	if !ok {
		return nil
	}
	var aliveSvcs []string
	for addr, t := range svcItem.Addresses {
		if t.Add(svcItem.updateInterval).After(time.Now()) {
			aliveSvcs = append(aliveSvcs, addr)
		} else {
			r.mu.Lock()
			delete(svcItem.Addresses, addr)
			r.mu.Unlock()
		}
	}
	return aliveSvcs
}
