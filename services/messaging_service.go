package services

import (
	"time"

	"github.com/mbocsi/gohab/proto"
	"github.com/mbocsi/gohab/server"
)

// MessagingServiceImpl implements MessagingService
type MessagingServiceImpl struct {
	broker       *server.Broker
	registry     *server.DeviceRegistry
	queryTracker *QueryTracker
	senderID     string
	handleFunc   func(proto.Message)
}

// NewMessagingService creates a new messaging service
func NewMessagingService(broker *server.Broker, registry *server.DeviceRegistry, handleFunc func(proto.Message), senderID string) MessagingService {
	return &MessagingServiceImpl{
		broker:       broker,
		registry:     registry,
		queryTracker: NewQueryTracker(30 * time.Second), // 30 second default timeout
		senderID:     senderID,
		handleFunc:   handleFunc,
	}
}

// SendQuery sends a query and waits for response
func (ms *MessagingServiceImpl) SendQuery(topic string, payload interface{}, timeout ...time.Duration) (*QueryResponse, error) {
	if err := validateTopic(topic); err != nil {
		return nil, err
	}
	
	sendFunc := func(msg proto.Message) error {
		ms.handleFunc(msg)
		return nil
	}
	
	return ms.queryTracker.SendQuery(topic, payload, sendFunc, timeout...)
}

// SendCommand sends a command message
func (ms *MessagingServiceImpl) SendCommand(topic string, payload interface{}) error {
	return ms.SendMessage(MessageRequest{
		Type:    "command",
		Topic:   topic,
		Payload: payload,
	})
}

// SendData sends a data message
func (ms *MessagingServiceImpl) SendData(topic string, payload interface{}) error {
	return ms.SendMessage(MessageRequest{
		Type:    "data",
		Topic:   topic,
		Payload: payload,
	})
}

// SendStatus sends a status message
func (ms *MessagingServiceImpl) SendStatus(topic string, payload interface{}) error {
	return ms.SendMessage(MessageRequest{
		Type:    "status",
		Topic:   topic,
		Payload: payload,
	})
}

// SendMessage sends a generic message
func (ms *MessagingServiceImpl) SendMessage(req MessageRequest) error {
	msg, err := createMessage(req, ms.senderID)
	if err != nil {
		return err
	}
	
	ms.handleFunc(msg)
	return nil
}

// Subscribe subscribes a client to a topic
func (ms *MessagingServiceImpl) Subscribe(topic string, client server.Client) error {
	if err := validateTopic(topic); err != nil {
		return err
	}
	
	ms.broker.Subscribe(topic, client)
	
	// Update client metadata
	meta := client.Meta()
	meta.Mu.Lock()
	meta.Subs[topic] = struct{}{}
	meta.Mu.Unlock()
	
	return nil
}

// Unsubscribe unsubscribes a client from a topic
func (ms *MessagingServiceImpl) Unsubscribe(topic string, client server.Client) error {
	if err := validateTopic(topic); err != nil {
		return err
	}
	
	ms.broker.Unsubscribe(topic, client)
	
	// Update client metadata
	meta := client.Meta()
	meta.Mu.Lock()
	delete(meta.Subs, topic)
	meta.Mu.Unlock()
	
	return nil
}

// HandleResponse handles incoming response messages for query correlation
func (ms *MessagingServiceImpl) HandleResponse(msg proto.Message) bool {
	return ms.queryTracker.HandleResponse(msg)
}