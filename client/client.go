package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/mbocsi/gohab/proto"
)

type Client struct {
	Name         string
	Connected    bool
	transport    Transport
	Id           string
	capabilities map[string]proto.Capability

	//Handlers for command messages
	commandHandlers map[string]func(proto.Message) error
	queryHandlers   map[string]func(proto.Message) (payload json.RawMessage, err error)

	// Helpers for publishing data or status
	dataFuncs   map[string]func(payload any) error
	statusFuncs map[string]func(payload any) error

	subHandlers map[string]func(proto.Message) error
}

func NewClient(name string, t Transport) *Client {
	return &Client{
		Name:            name,
		Connected:       false,
		transport:       t,
		capabilities:    make(map[string]proto.Capability),
		commandHandlers: make(map[string]func(proto.Message) error),
		queryHandlers:   make(map[string]func(proto.Message) (payload json.RawMessage, err error)),
		dataFuncs:       make(map[string]func(payload any) error),
		statusFuncs:     make(map[string]func(payload any) error),
		subHandlers:     make(map[string]func(proto.Message) error),
	}
}

func (c *Client) Start(addr string) error {
	setupLogger()

	err := c.transport.Connect(addr)
	if err != nil {
		return err
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
		c.readLoop()
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
			fmt.Println("read error:", err)
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

func (c *Client) readLoop() {
	for {
		msg, err := c.transport.Read()
		if err != nil {
			fmt.Println("read error:", err)
			return
		}
		slog.Debug("Message Received", "type", msg.Type, "topic", msg.Topic, "sender", msg.Sender, "size", len(msg.Payload))

		switch msg.Type {
		case "identify_ack":
			slog.Warn("Received unexpected identify_ack", "sender", msg.Sender)

		case "command":
			handler := c.commandHandlers[msg.Topic]
			if handler != nil {
				err := handler(msg)
				if err != nil {
					slog.Warn("An error occured in commandHandler", "topic", msg.Topic, "error", err.Error())
				}
			} else {
				slog.Warn("Topic not found in query handlers: Ignoring message", "topic", msg.Topic)
			}
		case "query":
			handler := c.queryHandlers[msg.Topic]
			if handler == nil {
				slog.Warn("Topic not found in query handlers")
				continue
			}
			responsePayload, err := handler(msg)
			if err != nil {
				slog.Warn("An error occured in queryHandler", "topic", msg.Topic, "error", err.Error())
				continue
			}
			responseMsg := proto.Message{Type: "response", Topic: msg.Topic, Recipient: msg.Sender, Payload: responsePayload, Timestamp: time.Now().Unix()}
			err = c.transport.Send(responseMsg)
			if err != nil {
				slog.Warn("An error occured when sending response", "response", responseMsg, "error", err.Error())
			}

		case "data", "status":
			handler, ok := c.subHandlers[msg.Topic]
			if !ok {
				slog.Warn("Topic not found in sub handlers")
			}
			err := handler(msg)
			if err != nil {
				slog.Warn("An error occured in subHandler", "topic", msg.Topic, "error", err.Error())
			}

		default:
			slog.Warn("Unhandled message", "type", msg.Type)
		}
	}
}

func (c *Client) sendIdentify() error {
	idPayload := proto.IdentifyPayload{
		ProposedName: c.Name,
		// TODO: Don't hardcode firmware
		Firmware:     "v1.0.0",
		Capabilities: slices.Collect(maps.Values(c.capabilities)),
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
	slog.Info("Sending identify message", "proposed_name", idPayload.ProposedName, "firmware", idPayload.Firmware, "capabilities", len(idPayload.Capabilities))
	return c.transport.Send(msg)
}

func (c *Client) AddCapability(cap proto.Capability) error {
	if err := cap.Validate(); err != nil {
		return err
	}
	c.capabilities[cap.Name] = cap
	return nil
}

func (c *Client) GetDataFunction(name string) (func(payload any) error, error) {
	cap, ok := c.capabilities[name]
	if !ok {
		return nil, fmt.Errorf("capability %q not found", name)
	}

	if !cap.Methods.Data.IsDefined() {
		return nil, fmt.Errorf("capability %q does not support data method", name)
	}

	fn := func(payload any) error {
		binPayload, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return c.transport.Send(proto.Message{
			Type:      "data",
			Topic:     fmt.Sprintf("%s/data", name),
			Payload:   binPayload,
			Timestamp: time.Now().Unix(),
		})
	}

	c.dataFuncs[name] = fn
	return fn, nil
}

func (c *Client) GetStatusFunction(name string) (func(payload any) error, error) {
	cap, ok := c.capabilities[name]
	if !ok {
		return nil, fmt.Errorf("capability %q not found", name)
	}

	if !cap.Methods.Status.IsDefined() {
		return nil, fmt.Errorf("capability %q does not support status method", name)
	}

	fn := func(payload any) error {
		binPayload, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return c.transport.Send(proto.Message{
			Type:      "status",
			Topic:     fmt.Sprintf("%s/status", name),
			Payload:   binPayload,
			Timestamp: time.Now().Unix(),
		})
	}

	c.statusFuncs[name] = fn
	return fn, nil
}

func (c *Client) RegisterCommandHandler(name string, handler func(msg proto.Message) error) error {
	cap, ok := c.capabilities[name]
	if !ok {
		return fmt.Errorf("capability %q not found", name)
	}

	if !cap.Methods.Command.IsDefined() {
		return fmt.Errorf("capability %q does not support command method", name)
	}
	if handler == nil {
		return fmt.Errorf("command handler must be provided for capability %q", name)
	}

	topic := fmt.Sprintf("%s/command", name)
	c.commandHandlers[topic] = handler
	return nil
}

func (c *Client) RegisterQueryHandler(name string, handler func(msg proto.Message) (json.RawMessage, error)) error {
	cap, ok := c.capabilities[name]
	if !ok {
		return fmt.Errorf("capability %q not found", name)
	}

	if !cap.Methods.Query.IsDefined() {
		return fmt.Errorf("capability %q does not support query method", name)
	}
	if handler == nil {
		return fmt.Errorf("query handler must be provided for capability %q", name)
	}

	topic := fmt.Sprintf("%s/query", name)
	c.queryHandlers[topic] = handler
	return nil
}

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
