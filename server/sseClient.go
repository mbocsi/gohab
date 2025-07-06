package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mbocsi/gohab/proto"
)

type SSEClient struct {
	ctx      context.Context
	writer   http.ResponseWriter
	flusher  http.Flusher
	topic    string
	template Templates
	DeviceMetadata
}

func NewSSEClient(ctx context.Context, w http.ResponseWriter, topic string, template Templates) *SSEClient {
	flusher, _ := w.(http.Flusher)
	return &SSEClient{
		ctx:      ctx,
		writer:   w,
		flusher:  flusher,
		topic:    topic,
		template: template,
		DeviceMetadata: DeviceMetadata{
			Id:   generateClientId("sse"),
			Name: "SSE Client",
			Subs: map[string]struct{}{topic: {}},
		},
	}
}

func (s *SSEClient) Send(msg proto.Message) error {
	var buf bytes.Buffer
	if err := s.template.templates.ExecuteTemplate(&buf, "message_toggle", msg); err != nil {
		return err
	}

	// Format for SSE (line breaks escaped)
	data := strings.ReplaceAll(buf.String(), "\n", "")
	fmt.Printf("%s\n", data)
	_, err := fmt.Fprintf(s.writer, "event: message\ndata: %s\n\n", data)
	s.flusher.Flush()
	return err
}

func (s *SSEClient) Meta() *DeviceMetadata {
	return &s.DeviceMetadata
}

func (s *SSEClient) MCPServer() *MCPServer {
	return nil
}
