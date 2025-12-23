package integration

import (
	"net"
	"testing"

	"github.com/mbocsi/gohab/client"
)

func getRandomPort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// Helper function to create clients with suppressed logging for integration tests
func newQuietClient(name string, transport client.Transport) *client.Client {
	return client.NewClientWithLogging(name, transport, client.SuppressedLogConfig())
}
