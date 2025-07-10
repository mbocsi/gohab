package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mbocsi/gohab/proto"
)

type LogConfig struct {
	Level   slog.Level
	Handler slog.Handler
	Output  io.Writer
}

type GohabServer struct {
	registery  *DeviceRegistry
	broker     *Broker
	transports map[string]Transport
	logConfig  *LogConfig

	topicSourcesMu sync.RWMutex
	topicSources   map[string]string // topic → deviceID
}

func NewGohabServer(registry *DeviceRegistry, broker *Broker) *GohabServer {
	return &GohabServer{
		registery:    registry,
		broker:       broker,
		transports:   make(map[string]Transport),
		topicSources: make(map[string]string),
	}
}

func NewGohabServerWithLogging(registry *DeviceRegistry, broker *Broker, logConfig *LogConfig) *GohabServer {
	return &GohabServer{
		registery:    registry,
		broker:       broker,
		transports:   make(map[string]Transport),
		topicSources: make(map[string]string),
		logConfig:    logConfig,
	}
}

// Interface methods for services
func (s *GohabServer) GetBroker() *Broker {
	return s.broker
}

func (s *GohabServer) GetRegistry() *DeviceRegistry {
	return s.registery
}

func (s *GohabServer) GetTransports() map[string]Transport {
	return s.transports
}

func (s *GohabServer) GetTopicSources() map[string]string {
	s.topicSourcesMu.RLock()
	defer s.topicSourcesMu.RUnlock()

	result := make(map[string]string)
	for topic, deviceID := range s.topicSources {
		result[topic] = deviceID
	}
	return result
}

func (s *GohabServer) SetLogConfig(config *LogConfig) {
	s.logConfig = config
}

func (s *GohabServer) RegisterTransport(t Transport) {
	t.OnMessage(s.Handle)
	t.OnConnect(s.RegisterDevice)
	t.OnDisconnect(s.RemoveDevice)

	// Get transport metadata and use ID as key
	meta := t.Meta()
	if meta.ID == "" {
		// Generate an ID if not set
		meta.ID = meta.Protocol + "-" + meta.Address
	}

	s.transports[meta.ID] = t
}

func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:  slog.LevelInfo,
		Output: os.Stdout,
	}
}

func QuietLogConfig() *LogConfig {
	return &LogConfig{
		Level:  slog.LevelError,
		Output: os.Stdout,
	}
}

func SuppressedLogConfig() *LogConfig {
	return &LogConfig{
		Level:  slog.LevelError,
		Output: io.Discard,
	}
}

func (s *GohabServer) setupLogger() {
	var handler slog.Handler

	if s.logConfig == nil {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		output := s.logConfig.Output
		if output == nil {
			output = os.Stdout
		}

		if s.logConfig.Handler != nil {
			handler = s.logConfig.Handler
		} else {
			handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
				Level: s.logConfig.Level,
			})
		}
	}

	slog.SetDefault(slog.New(handler))
}

func (s *GohabServer) Start() error {
	s.setupLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s.start(ctx)
	return nil
}

func (s *GohabServer) start(ctx context.Context) error {
	// TODO: Add context to check if go routines exit for some reason
	for _, t := range s.transports {
		go t.Start()
	}

	<-ctx.Done()
	slog.Info("Shutting down transports and servers")

	for _, t := range s.transports {
		if err := t.Shutdown(); err != nil {
			slog.Error("There was an error when shutting down transport server", "error", err.Error())
		}
	}
	return nil
}

func (s *GohabServer) RegisterDevice(client Client) error {
	s.registery.Store(client)

	slog.Info("Registered client", "id", client.Meta().Id)
	return nil
}

func (s *GohabServer) RemoveDevice(client Client) {
	s.registery.Delete(client.Meta().Id)

	s.topicSourcesMu.Lock()
	for _, feature := range client.Meta().Features {
		if _, ok := s.topicSources[feature.Name]; !ok {
			slog.Error("Feature name/topic does not exist in topic sources", "topic", feature.Name)
			continue
		}
		delete(s.topicSources, feature.Name)
	}
	s.topicSourcesMu.Unlock()
}

// Message handling methods
func (s *GohabServer) Handle(msg proto.Message) {
	switch msg.Type {
	case "identify":
		s.handleIdentify(msg)

	case "data", "status":
		s.handleData(msg)

	case "subscribe", "unsubscribe":
		s.handleSubscription(msg)

	case "command":
		s.handleCommand(msg)

	case "query", "response":
		s.handleQuery(msg)

	default:
		slog.Warn("Unhandled message type", "type", msg.Type, "sender", msg.Sender)
	}
}

// ---------- identify ---------- //

func (s *GohabServer) handleIdentify(msg proto.Message) {
	id := msg.Sender
	client, ok := s.registery.Get(id)
	if !ok {
		slog.Error("Unknown client sent identify message", "id", id)
		return
	}

	var idPayload proto.IdentifyPayload
	if err := json.Unmarshal(msg.Payload, &idPayload); err != nil {
		slog.Warn("Invalid JSON identify payload received", "id", id, "error", err.Error(), "data", string(msg.Payload))
		return
	}

	features := make(map[string]proto.Feature)
	for _, feature := range idPayload.Features {
		features[feature.Name] = feature
	}

	client.Meta().Mu.Lock()
	client.Meta().Id = id
	client.Meta().Name = idPayload.ProposedName
	client.Meta().Firmware = idPayload.Firmware
	client.Meta().Features = features
	client.Meta().Mu.Unlock()

	s.topicSourcesMu.Lock()
	for _, feature := range client.Meta().Features {
		if _, ok := s.topicSources[feature.Name]; ok {
			slog.Error("Feature name/topic already exists in system: skipping source registration", "topic", feature.Name)
			continue
		}
		s.topicSources[feature.Name] = client.Meta().Id
	}
	s.topicSourcesMu.Unlock()

	ackPayload := proto.IdAckPayload{
		AssignedId: client.Meta().Id,
		Status:     "ok",
	}

	ackPayloadBytes, err := json.Marshal(ackPayload)
	if err != nil {
		slog.Warn("There was an error marshalling ack payload", "error", err.Error())
		return
	}

	ack := proto.Message{
		Type:      "identify_ack",
		Payload:   ackPayloadBytes,
		Sender:    "server",
		Timestamp: time.Now().Unix(),
	}
	slog.Info("Identified client", "id", id)
	client.Send(ack)
}

// ---------- data / status ---------- //

func (s *GohabServer) handleData(msg proto.Message) {
	// Fan-out to all subscribers of msg.Topic.
	//
	//  ⚠  json.RawMessage *is already* a []byte alias,
	//     so we can pass it straight to Publish.
	s.broker.Publish(msg)
}

// ---------- stubs for other message kinds ---------- //

func (s *GohabServer) handleSubscription(msg proto.Message) {
	switch msg.Type {
	case "subscribe":
		client, ok := s.registery.Get(msg.Sender)
		if !ok {
			slog.Warn("Client ID not found", "client", msg.Sender)
			return
		}
		var sub proto.SubscriptionPayload
		err := json.Unmarshal(msg.Payload, &sub)
		if err != nil {
			slog.Warn("Error unmarshalling subscribe payload", "error", err.Error())
			return
		}
		client.Meta().Mu.Lock()
		for _, topic := range sub.Topics {
			s.broker.Subscribe(topic, client)
			client.Meta().Subs[topic] = struct{}{}
		}
		client.Meta().Mu.Unlock()

	case "unsubscribe":
		client, ok := s.registery.Get(msg.Sender)
		if !ok {
			slog.Warn("Client ID not found", "client", msg.Sender)
			return
		}
		var sub proto.SubscriptionPayload
		err := json.Unmarshal(msg.Payload, &sub)
		if err != nil {
			slog.Warn("Error unmarshalling unsubscribe payload", "error", err.Error())
			return
		}
		client.Meta().Mu.Lock()
		for _, topic := range sub.Topics {
			s.broker.Unsubscribe(topic, client)
			delete(client.Meta().Subs, topic)
		}
		client.Meta().Mu.Unlock()
	}
}

// TODO: Implement these
func (s *GohabServer) handleCommand(msg proto.Message) {
	s.topicSourcesMu.RLock()
	if id, ok := s.topicSources[msg.Topic]; !ok {
		slog.Warn("No source found for the commanded topic", "topic", msg.Topic)
	} else {
		if client, ok := s.registery.Get(id); !ok {
			slog.Error("Client ID not found", "id", id)
		} else {
			err := client.Send(msg)
			if err != nil {
				slog.Error("Error when forwarding message", "error", err.Error())
			}
		}
	}
	s.topicSourcesMu.RUnlock()
}

// TODO: This looks nasty
func (s *GohabServer) handleQuery(msg proto.Message) {
	if msg.Type == "query" {
		s.topicSourcesMu.RLock()
		if id, ok := s.topicSources[msg.Topic]; !ok {
			slog.Warn("No source found for the queried topic", "topic", msg.Topic)
		} else {
			if client, ok := s.registery.Get(id); !ok {
				slog.Error("Client ID not found", "id", id)
			} else {
				err := client.Send(msg)
				if err != nil {
					slog.Error("Error when forwarding message", "error", err.Error())
				}
			}
		}
		s.topicSourcesMu.RUnlock()
	} else {
		if msg.Recipient == "" {
			slog.Warn("Recipient is missing in response message", "sender", msg.Sender)
			return
		}
		client, ok := s.registery.Get(msg.Recipient)
		if !ok {
			slog.Warn("Recipient not found in system", "recipient", msg.Recipient)
			return
		}

		err := client.Send(msg)
		if err != nil {
			slog.Error("An error occured when sending response", "sender", msg.Sender, "recipient", client.Meta().Id)
		}
	}
}
