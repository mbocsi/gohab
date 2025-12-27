package client

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/hashicorp/mdns"
)

// DiscoveredService represents a discovered GoHab server
type DiscoveredService struct {
	ServiceName string
	Address     string
	Port        int
	Transport   string // "tcp" or "websocket"
	TXTRecords  []string
}

// discoverService discovers a specific GoHab service type using mDNS
func discoverService(serviceType string, timeout time.Duration) (*DiscoveredService, error) {
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	entriesCh := make(chan *mdns.ServiceEntry, 4)

	// Start discovery in background
	go func() {
		defer close(entriesCh)
		mdns.Lookup(serviceType, entriesCh)
	}()

	// Wait for first result or timeout
	select {
	case entry := <-entriesCh:
		if entry == nil {
			return nil, fmt.Errorf("no %s service found", serviceType)
		}

		var address string
		if entry.AddrV4 != nil {
			address = entry.AddrV4.String()
		} else if entry.AddrV6 != nil {
			address = fmt.Sprintf("[%s]", entry.AddrV6.String())
		} else {
			return nil, fmt.Errorf("no valid address found for service")
		}

		var transport string
		if serviceType == "_gohab-tcp._tcp" {
			transport = "tcp"
		} else if serviceType == "_gohab-ws._tcp" {
			transport = "websocket"
		}

		service := &DiscoveredService{
			ServiceName: entry.Name,
			Address:     address,
			Port:        entry.Port,
			Transport:   transport,
			TXTRecords:  entry.InfoFields,
		}

		slog.Info("Discovered GoHab server",
			"service_name", service.ServiceName,
			"address", service.Address,
			"port", service.Port,
			"transport", service.Transport,
		)

		return service, nil

	case <-time.After(timeout):
		return nil, fmt.Errorf("mDNS discovery timeout for %s", serviceType)
	}
}

// DiscoverTCPService discovers the first available TCP GoHab server
func DiscoverTCPService(timeout time.Duration) (*DiscoveredService, error) {
	return discoverService("_gohab-tcp._tcp", timeout)
}

// DiscoverWebSocketService discovers the first available WebSocket GoHab server
func DiscoverWebSocketService(timeout time.Duration) (*DiscoveredService, error) {
	return discoverService("_gohab-ws._tcp", timeout)
}
