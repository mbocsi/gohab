package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/mbocsi/gohab/proto"
)

type Client struct {
	Name      string
	Connected bool
	transport Transport
	Id        string

	// Features
	featMu        sync.RWMutex
	features map[string]proto.Feature

	// Handlers
	handlerMu       sync.RWMutex
	commandHandlers map[string]func(proto.Message) error
	queryHandlers   map[string]func(proto.Message) (any, error)
	subHandlers     map[string]func(proto.Message) error

	// Request/response channels
	resMu    sync.Mutex
	resChans map[string]chan proto.Message

	// Publisher functions
	funcMu      sync.RWMutex
	dataFuncs   map[string]func(payload any) error
	statusFuncs map[string]func(payload any) error
}

func NewClient(name string, t Transport) *Client {
	return &Client{
		Name:            name,
		Connected:       false,
		transport:       t,
		features:    make(map[string]proto.Feature),
		commandHandlers: make(map[string]func(proto.Message) error),
		queryHandlers:   make(map[string]func(proto.Message) (payload any, err error)),
		resChans:        make(map[string]chan proto.Message),
		dataFuncs:       make(map[string]func(payload any) error),
		statusFuncs:     make(map[string]func(payload any) error),
		subHandlers:     make(map[string]func(proto.Message) error),
	}
}

func (c *Client) Start(addr string) error {
	setupLogger()

reconnect:
	err := c.transport.Connect(addr)
	if err != nil {
		slog.Error("Failed to connect, retrying in 2 seconds", "error", err)
		time.Sleep(2 * time.Second)
		goto reconnect
	}
	c.Connected = true
	ackCh := make(chan struct{})
	defer close(ackCh)
	// Start identify loop
	go c.identify(ackCh)

	// Wait for identify_ack or timeout
	select {
	case <-ackCh:
		topics := slices.Collect(maps.Keys(c.subHandlers))
		msg, err := createSubMessage(topics)
		if err != nil {
			return err
		}
		err = c.transport.Send(msg)
		if err != nil {
			return err
		}
		err = c.readLoop()
		if err != nil {
			c.Connected = false
			slog.Error("Connection lost, reconnecting", "error", err)
			time.Sleep(2 * time.Second)
			goto reconnect
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for identify_ack")
	}
	return nil
}

func (c *Client) identify(ackCh chan struct{}) {
	const maxRetries = 3
	retries := 0

retryIdentify:
	if err := c.sendIdentify(); err != nil {
		slog.Error("Failed to send identify", "err", err)
		return
	}
	for {
		msg, err := c.transport.Read()
		if err != nil {
			slog.Error("read error", "error", err.Error())
			return
		}
		slog.Debug("Message Received", "type", msg.Type, "topic", msg.Topic, "sender", msg.Sender, "size", len(msg.Payload))

		switch msg.Type {
		case "identify_ack":
			var idAckPayload proto.IdAckPayload
			err := json.Unmarshal(msg.Payload, &idAckPayload)
			if err != nil {
				slog.Warn("Invalid JSON identify acknowledge payload", "error", err.Error(), "payload", string(msg.Payload))
				continue
			}
			if idAckPayload.Status != "ok" {
				slog.Warn("Server rejected identify", "status", idAckPayload.Status, "payload", string(msg.Payload))
				retries++
				if retries >= maxRetries {
					slog.Error("Max retries reached. Giving up.")
					return
				}

				time.Sleep(1 * time.Second)
				goto retryIdentify
			}
			c.Id = idAckPayload.AssignedId
			close(ackCh) // unblock Start
			return
		default:
			slog.Warn("Received a message other than identify_ack")
		}
	}
}

func (c *Client) readLoop() error {
	for {
		msg, err := c.transport.Read()
		if err != nil {
			slog.Error("read error", "error", err.Error())
			return err
		}
		slog.Debug("Message Received", "type", msg.Type, "topic", msg.Topic, "sender", msg.Sender, "size", len(msg.Payload))

		switch msg.Type {
		case "identify_ack":
			slog.Warn("Received unexpected identify_ack", "sender", msg.Sender)

		case "command":
			c.handlerMu.RLock()
			handler := c.commandHandlers[msg.Topic]
			c.handlerMu.RUnlock()
			if handler != nil {
				err := handler(msg)
				if err != nil {
					slog.Warn("An error occured in commandHandler", "topic", msg.Topic, "error", err.Error())
				}
			} else {
				slog.Warn("Topic not found in query handlers: Ignoring message", "topic", msg.Topic)
			}
		case "query":
			c.handlerMu.RLock()
			handler := c.queryHandlers[msg.Topic]
			c.handlerMu.RUnlock()
			if handler == nil {
				slog.Warn("Topic not found in query handlers")
				continue
			}
			response, err := handler(msg)
			if err != nil {
				slog.Warn("An error occured in queryHandler", "topic", msg.Topic, "error", err.Error())
				continue
			}
			responsePayload, err := json.Marshal(response)
			if err != nil {
				slog.Warn("An error occured when marshalling response", "topic", msg.Topic, "error", err.Error())
				continue
			}
			responseMsg := proto.Message{Type: "response", Topic: msg.Topic, Recipient: msg.Sender, Payload: responsePayload, Timestamp: time.Now().Unix()}
			err = c.transport.Send(responseMsg)
			if err != nil {
				slog.Warn("An error occured when sending response", "response", responseMsg, "error", err.Error())
			}

		case "data", "status":
			c.handlerMu.RLock()
			handler, ok := c.subHandlers[msg.Topic]
			c.handlerMu.RUnlock()
			if !ok {
				slog.Warn("Topic not found in sub handlers")
			}
			err := handler(msg)
			if err != nil {
				slog.Warn("An error occured in subHandler", "topic", msg.Topic, "error", err.Error())
			}

		case "response":
			c.resMu.Lock()
			ch, ok := c.resChans[msg.Topic]
			if ok {
				ch <- msg
				close(ch)
				delete(c.resChans, msg.Topic)
			}
			c.resMu.Unlock()

		default:
			slog.Warn("Unhandled message", "type", msg.Type)
		}
	}
}

func (c *Client) SendQuery(topic string, payload any) (proto.Message, error) {

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return proto.Message{}, err
	}

	msg := proto.Message{
		Type:    "query",
		Topic:   topic,
		Payload: rawPayload,
	}

	// Set up a channel to receive the response
	respChan := make(chan proto.Message, 1)

	c.resMu.Lock()
	c.resChans[topic] = respChan
	c.resMu.Unlock()

	err = c.transport.Send(msg)
	if err != nil {
		return proto.Message{}, err
	}

	// Wait for the response or timeout
	select {
	case resp := <-respChan:
		return resp, nil
	case <-time.After(5 * time.Second):
		return proto.Message{}, fmt.Errorf("timeout waiting for response")
	}
}

func (c *Client) SendCommand(topic string, payload any) error {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := proto.Message{
		Type:    "command",
		Topic:   topic,
		Payload: rawPayload,
	}
	err = c.transport.Send(msg)
	return err
}

func (c *Client) sendIdentify() error {
	idPayload := proto.IdentifyPayload{
		ProposedName: c.Name,
		// TODO: Don't hardcode firmware
		Firmware:     "v1.0.0",
		Features: slices.Collect(maps.Values(c.features)),
	}
	payload, err := json.Marshal(idPayload)
	if err != nil {
		return err
	}
	msg := proto.Message{
		Type:      "identify",
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}
	slog.Info("Sending identify message", "proposed_name", idPayload.ProposedName, "firmware", idPayload.Firmware, "features", len(idPayload.Features))
	return c.transport.Send(msg)
}

func (c *Client) AddFeature(feat proto.Feature) error {
	if err := feat.Validate(); err != nil {
		return err
	}
	c.features[feat.Name] = feat
	return nil
}

func (c *Client) GetDataFunction(name string) (func(payload any) error, error) {
	feat, ok := c.features[name]
	if !ok {
		return nil, fmt.Errorf("feature %q not found", name)
	}

	if !feat.Methods.Data.IsDefined() {
		return nil, fmt.Errorf("feature %q does not support data method", name)
	}

	fn := func(payload any) error {
		binPayload, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return c.transport.Send(proto.Message{
			Type:      "data",
			Topic:     name,
			Payload:   binPayload,
			Timestamp: time.Now().Unix(),
		})
	}

	c.dataFuncs[name] = fn
	return fn, nil
}

func (c *Client) GetStatusFunction(name string) (func(payload any) error, error) {
	feat, ok := c.features[name]
	if !ok {
		return nil, fmt.Errorf("feature %q not found", name)
	}

	if !feat.Methods.Status.IsDefined() {
		return nil, fmt.Errorf("feature %q does not support status method", name)
	}

	fn := func(payload any) error {
		binPayload, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return c.transport.Send(proto.Message{
			Type:      "status",
			Topic:     name,
			Payload:   binPayload,
			Timestamp: time.Now().Unix(),
		})
	}

	c.statusFuncs[name] = fn
	return fn, nil
}

func (c *Client) RegisterCommandHandler(name string, handler func(msg proto.Message) error) error {
	feat, ok := c.features[name]
	if !ok {
		return fmt.Errorf("feature %q not found", name)
	}

	if !feat.Methods.Command.IsDefined() {
		return fmt.Errorf("feature %q does not support command method", name)
	}
	if handler == nil {
		return fmt.Errorf("command handler must be provided for feature %q", name)
	}

	c.commandHandlers[name] = handler
	return nil
}

func (c *Client) RegisterQueryHandler(name string, handler func(msg proto.Message) (any, error)) error {
	feat, ok := c.features[name]
	if !ok {
		return fmt.Errorf("feature %q not found", name)
	}

	if !feat.Methods.Query.IsDefined() {
		return fmt.Errorf("feature %q does not support query method", name)
	}
	if handler == nil {
		return fmt.Errorf("query handler must be provided for feature %q", name)
	}

	c.queryHandlers[name] = handler
	return nil
}

// Subscribes to all methods (data & status)
func (c *Client) Subscribe(topic string, callbackFn func(msg proto.Message) error) error {
	c.subHandlers[topic] = callbackFn

	subscribeMsg, err := createSubMessage([]string{topic})
	if err != nil {
		return err
	}

	if !c.Connected {
		return nil
	}

	return c.transport.Send(subscribeMsg)
}

func createSubMessage(topics []string) (proto.Message, error) {
	subPayload := proto.SubscriptionPayload{
		Topics: topics,
	}

	payload, err := json.Marshal(subPayload)
	if err != nil {
		return proto.Message{}, err
	}

	return proto.Message{
		Type:      "subscribe",
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}, nil
}

func (c *Client) Unsubscribe(topic string) error {
	delete(c.subHandlers, topic)

	unsubscribeMsg, err := createUnsubMessage([]string{topic})
	if err != nil {
		return err
	}

	if !c.Connected {
		return nil
	}

	return c.transport.Send(unsubscribeMsg)
}

func createUnsubMessage(topics []string) (proto.Message, error) {
	subPayload := proto.SubscriptionPayload{
		Topics: topics,
	}

	payload, err := json.Marshal(subPayload)
	if err != nil {
		return proto.Message{}, err
	}

	return proto.Message{
		Type:      "unsubscribe",
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}, nil
}

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic("marshal error: " + err.Error())
	}
	return data
}

func setupLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}
