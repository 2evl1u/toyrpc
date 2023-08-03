package toyrpc

import "time"

type ServiceItem struct {
	Addresses   []string
	lastUpdated time.Time
}

type Registry struct {
	services map[string]*ServiceItem
}
