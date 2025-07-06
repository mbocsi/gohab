package server

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mbocsi/gohab/proto"
)

func (c *Coordinator) Handle(msg proto.Message) {
	switch msg.Type {
	case "identify":
		c.handleIdentify(msg)

	case "data", "status":
		c.handleData(msg)

	case "subscribe", "unsubscribe":
		c.handleSubscription(msg)

	case "command":
		c.handleCommand(msg)

	case "query", "response":
		c.handleQuery(msg)

	default:
		slog.Warn("Unhandled message type", "type", msg.Type, "sender", msg.Sender)
	}
}

// ---------- identify ---------- //

func (c *Coordinator) handleIdentify(msg proto.Message) {
	id := msg.Sender
	client, ok := c.Registery.Get(id)
	if !ok {
		slog.Error("Unknown client sent identify message", "id", id)
		return
	}

	var idPayload proto.IdentifyPayload
	if err := json.Unmarshal(msg.Payload, &idPayload); err != nil {
		slog.Warn("Invalid JSON identify payload received", "id", id, "error", err.Error(), "data", string(msg.Payload))
		return
	}

	capabilities := make(map[string]proto.Capability)
	for _, capability := range idPayload.Capabilities {
		capabilities[capability.Name] = capability
	}

	client.Meta().Mu.Lock()
	client.Meta().Id = id
	client.Meta().Name = idPayload.ProposedName
	client.Meta().Firmware = idPayload.Firmware
	client.Meta().Capabilities = capabilities
	client.Meta().Mu.Unlock()

	c.topicSourcesMu.Lock()
	for _, capability := range client.Meta().Capabilities {
		if _, ok := c.topicSources[capability.Name]; ok {
			slog.Error("Capability name/topic already exists in system: skipping source registration", "topic", capability.Name)
			continue
		}
		c.topicSources[capability.Name] = client.Meta().Id
	}
	c.topicSourcesMu.Unlock()

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

func (c *Coordinator) handleData(msg proto.Message) {
	// Fan-out to all subscribers of msg.Topic.
	//
	//  ⚠  json.RawMessage *is already* a []byte alias,
	//     so we can pass it straight to Publish.
	c.Broker.Publish(msg)
}

// ---------- stubs for other message kinds ---------- //

func (c *Coordinator) handleSubscription(msg proto.Message) {
	switch msg.Type {
	case "subscribe":
		client, ok := c.Registery.Get(msg.Sender)
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
			c.Broker.Subscribe(topic, client)
			client.Meta().Subs[topic] = struct{}{}
		}
		client.Meta().Mu.Unlock()

	case "unsubscribe":
		client, ok := c.Registery.Get(msg.Sender)
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
			c.Broker.Unsubscribe(topic, client)
			delete(client.Meta().Subs, topic)
		}
		client.Meta().Mu.Unlock()
	}
}

// TODO: Implement these
func (c *Coordinator) handleCommand(msg proto.Message) {
	c.topicSourcesMu.RLock()
	if id, ok := c.topicSources[msg.Topic]; !ok {
		slog.Warn("No source found for the commanded topic", "topic", msg.Topic)
	} else {
		if client, ok := c.Registery.Get(id); !ok {
			slog.Error("Client ID not found", "id", id)
		} else {
			err := client.Send(msg)
			if err != nil {
				slog.Error("Error when forwarding message", "error", err.Error())
			}
		}
	}
	c.topicSourcesMu.RUnlock()
}

// TODO: This looks nasty
func (c *Coordinator) handleQuery(msg proto.Message) {
	if msg.Type == "query" {
		c.topicSourcesMu.RLock()
		if id, ok := c.topicSources[msg.Topic]; !ok {
			slog.Warn("No source found for the queried topic", "topic", msg.Topic)
		} else {
			if client, ok := c.Registery.Get(id); !ok {
				slog.Error("Client ID not found", "id", id)
			} else {
				err := client.Send(msg)
				if err != nil {
					slog.Error("Error when forwarding message", "error", err.Error())
				}
			}
		}
		c.topicSourcesMu.RUnlock()
	} else {
		if msg.Recipient == "" {
			slog.Warn("Recipient is missing in response message", "sender", msg.Sender)
			return
		}
		client, ok := c.Registery.Get(msg.Recipient)
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
