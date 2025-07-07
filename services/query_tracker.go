package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/mbocsi/gohab/proto"
	"github.com/google/uuid"
)

// QueryTracker manages query-response correlation
type QueryTracker struct {
	queries map[string]chan QueryResponse
	timeout time.Duration
	mu      sync.RWMutex
}

// NewQueryTracker creates a new query tracker
func NewQueryTracker(defaultTimeout time.Duration) *QueryTracker {
	return &QueryTracker{
		queries: make(map[string]chan QueryResponse),
		timeout: defaultTimeout,
	}
}

// SendQuery sends a query and waits for response
func (qt *QueryTracker) SendQuery(topic string, payload interface{}, sendFunc func(proto.Message) error, timeout ...time.Duration) (*QueryResponse, error) {
	queryID := uuid.New().String()
	responseChan := make(chan QueryResponse, 1)
	
	// Determine timeout
	queryTimeout := qt.timeout
	if len(timeout) > 0 && timeout[0] > 0 {
		queryTimeout = timeout[0]
	}
	
	// Register query
	qt.mu.Lock()
	qt.queries[queryID] = responseChan
	qt.mu.Unlock()
	
	// Clean up on completion
	defer func() {
		qt.mu.Lock()
		delete(qt.queries, queryID)
		close(responseChan)
		qt.mu.Unlock()
	}()
	
	// Create and send query message
	payloadBytes, err := marshalPayload(payload)
	if err != nil {
		return nil, ServiceError{
			Code:    ErrCodeInvalidInput,
			Message: "Failed to marshal query payload",
			Cause:   err,
		}
	}
	
	queryMsg := proto.Message{
		Type:      "query",
		Topic:     topic,
		Payload:   payloadBytes,
		Sender:    "web-ui",
		Recipient: queryID, // Use queryID as recipient for correlation
		Timestamp: time.Now().Unix(),
	}
	
	// Send query
	if err := sendFunc(queryMsg); err != nil {
		return nil, ServiceError{
			Code:    ErrCodeInternal,
			Message: "Failed to send query message",
			Cause:   err,
		}
	}
	
	// Wait for response or timeout
	select {
	case response := <-responseChan:
		return &response, response.Error
	case <-time.After(queryTimeout):
		return nil, ServiceError{
			Code:    ErrCodeTimeout,
			Message: fmt.Sprintf("Query timeout after %v", queryTimeout),
		}
	}
}

// HandleResponse processes incoming response messages
func (qt *QueryTracker) HandleResponse(msg proto.Message) bool {
	if msg.Type != "response" {
		return false
	}
	
	// Extract query ID from recipient field
	queryID := msg.Recipient
	if queryID == "" {
		return false
	}
	
	qt.mu.RLock()
	responseChan, exists := qt.queries[queryID]
	qt.mu.RUnlock()
	
	if !exists {
		return false
	}
	
	// Send response to waiting query
	response := QueryResponse{
		QueryID:   queryID,
		Response:  msg,
		Timestamp: time.Now(),
	}
	
	select {
	case responseChan <- response:
		return true
	default:
		// Channel full or closed
		return false
	}
}

// Cleanup removes expired queries
func (qt *QueryTracker) Cleanup() {
	qt.mu.Lock()
	defer qt.mu.Unlock()
	
	// This would be called periodically to clean up abandoned queries
	// For now, we rely on the defer cleanup in SendQuery
}