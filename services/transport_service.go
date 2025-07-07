package services

import (
	"github.com/mbocsi/gohab/server"
)

// TransportServiceImpl implements TransportService
type TransportServiceImpl struct {
	transports []server.Transport
}

// NewTransportService creates a new transport service
func NewTransportService(transports []server.Transport) TransportService {
	return &TransportServiceImpl{
		transports: transports,
	}
}

// ListTransports returns all transport information
func (ts *TransportServiceImpl) ListTransports() ([]TransportInfo, error) {
	result := make([]TransportInfo, 0, len(ts.transports))
	
	for i, transport := range ts.transports {
		result = append(result, convertTransportMeta(i, transport))
	}
	
	return result, nil
}

// GetTransport returns a specific transport by index
func (ts *TransportServiceImpl) GetTransport(index int) (*TransportInfo, error) {
	if index < 0 || index >= len(ts.transports) {
		return nil, ServiceError{
			Code:    ErrCodeNotFound,
			Message: "Transport index out of range",
		}
	}
	
	info := convertTransportMeta(index, ts.transports[index])
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