package services

import (
	"github.com/mbocsi/gohab/server"
)

// TransportServiceImpl implements TransportService
type TransportServiceImpl struct {
	transports map[string]server.Transport
}

// NewTransportService creates a new transport service
func NewTransportService(transports map[string]server.Transport) TransportService {
	return &TransportServiceImpl{
		transports: transports,
	}
}

// ListTransports returns all transport information
func (ts *TransportServiceImpl) ListTransports() ([]TransportInfo, error) {
	result := make([]TransportInfo, 0, len(ts.transports))
	
	for _, transport := range ts.transports {
		result = append(result, convertTransportMeta(transport))
	}
	
	return result, nil
}

// GetTransport returns a specific transport by ID
func (ts *TransportServiceImpl) GetTransport(id string) (*TransportInfo, error) {
	transport, exists := ts.transports[id]
	if !exists {
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Transport not found: " + id,
		}
	}
	
	info := convertTransportMeta(transport)
	return &info, nil
}

// GetTransportStats returns aggregate transport statistics
func (ts *TransportServiceImpl) GetTransportStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	totalTransports := len(ts.transports)
	connectedTransports := 0
	totalConnections := 0
	
	for _, transport := range ts.transports {
		meta := transport.Meta()
		if meta.Connected {
			connectedTransports++
		}
		totalConnections += len(meta.Clients)
	}
	
	stats["total_transports"] = totalTransports
	stats["connected_transports"] = connectedTransports
	stats["total_connections"] = totalConnections
	
	return stats, nil
}