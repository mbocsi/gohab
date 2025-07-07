package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mbocsi/gohab/proto"
)

type GohabServer struct {
	registery  *DeviceRegistry
	broker     *Broker
	transports []Transport
	templates  *Templates

	topicSourcesMu sync.RWMutex
	topicSources   map[string]string // topic → deviceID
}

func NewGohabServer(registry *DeviceRegistry, broker *Broker) *GohabServer {
	return &GohabServer{
		registery:    registry,
		broker:       broker,
		templates:    NewTemplates("templates/layout/*.html"),
		topicSources: make(map[string]string),
	}
}

func (s *GohabServer) RegisterTransport(t Transport) {
	t.OnMessage(s.Handle)
	t.OnConnect(s.RegisterDevice)
	t.OnDisconnect(s.RemoveDevice)
	s.transports = append(s.transports, t)
}

func setupLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}

func (s *GohabServer) Start(addr string) error {
	setupLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	s.start(ctx, addr)
	return nil
}

func (s *GohabServer) start(ctx context.Context, addr string) error {
	// TODO: Add context to check if go routines exit for some reason
	for _, t := range s.transports {
		go t.Start()
	}

	server := &http.Server{
		Addr:    addr, // configurable
		Handler: s.Routes(),
	}

	slog.Info("Starting http server", "addr", addr)
	go server.ListenAndServe()

	<-ctx.Done()
	slog.Info("Shutting down transports and servers")

	slog.Info("Shutting down http server", "addr", addr)
	err := server.Shutdown(context.Background())
	if err != nil {
		slog.Error("There wan an error when shutting down the Web Server", "error", err.Error())
	}

	for _, t := range s.transports {
		if err = t.Shutdown(); err != nil {
			slog.Error("There was an error when shutting down transport server", "error", err.Error())
		}
	}
	return nil
}

func (s *GohabServer) Routes() http.Handler {
	r := chi.NewRouter()
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	r.Get("/", s.HandleHome)
	r.Get("/devices", s.HandleDevices)
	r.Get("/devices/{id}", s.HandleDeviceDetail)
	r.Get("/features", s.HandleFeatures)
	r.Get("/features/{name}", s.HandleFeatureDetail)
	r.Get("/transports", s.HandleTransports)
	r.Get("/transports/{i}", s.HandleTransportDetail)
	return r
}

func (s *GohabServer) RegisterDevice(client Client) error {
	s.registery.Store(client)

	slog.Info("Registered client", "id", client.Meta().Id)
	return nil
}

func (s *GohabServer) RemoveDevice(client Client) {
	s.registery.Delete(client.Meta().Id)

	s.topicSourcesMu.Lock()
	for _, capability := range client.Meta().Capabilities {
		if _, ok := s.topicSources[capability.Name]; !ok {
			slog.Error("Capability name/topic does not exist in topic sources", "topic", capability.Name)
			continue
		}
		delete(s.topicSources, capability.Name)
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

	s.topicSourcesMu.Lock()
	for _, capability := range client.Meta().Capabilities {
		if _, ok := s.topicSources[capability.Name]; ok {
			slog.Error("Capability name/topic already exists in system: skipping source registration", "topic", capability.Name)
			continue
		}
		s.topicSources[capability.Name] = client.Meta().Id
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

// Web handler methods
func (s *GohabServer) HandleDeviceDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	device, ok := s.registery.Get(id)
	if !ok {
		http.NotFound(w, r)
		slog.Warn("Device id not found in registery", "id", id)
		return
	}
	if _, ok := r.Header["Hx-Request"]; !ok {

		clone, err := s.templates.ExtendedTemplates("devices")
		if err != nil {
			slog.Error("Error when extending templates", "page", "devices")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		devices := s.registery.List()
		clone.RenderPage(w, "device_detail", map[string]interface{}{
			"Device":  device.Meta(),
			"Devices": devices,
		})
	} else {
		clone, err := s.templates.ExtendedTemplates("device_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "device_detail")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		clone.Render(w, "content", map[string]interface{}{
			"Device": device.Meta(),
		})
	}
}

func (s *GohabServer) HandleDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.registery.List()
	s.templates.RenderPage(w, "devices", map[string]interface{}{
		"Devices": devices,
	})
}

func (s *GohabServer) HandleFeatures(w http.ResponseWriter, r *http.Request) {
	s.topicSourcesMu.RLock()
	s.templates.RenderPage(w, "features", map[string]any{
		"Features": s.topicSources,
	})
	s.topicSourcesMu.RUnlock()
}

func (s *GohabServer) HandleFeatureDetail(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "name")
	s.topicSourcesMu.RLock()
	sourceId, ok := s.topicSources[topic]
	s.topicSourcesMu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		slog.Warn("Feature not found in topicSources", "feature", topic)
		return
	}

	client, ok := s.registery.Get(sourceId)
	if !ok {
		http.NotFound(w, r)
		slog.Error("Client not found in registery", "client", sourceId)
		return
	}
	s.broker.mu.RLock()
	subs := slices.Collect(maps.Keys(s.broker.subs[topic]))
	s.broker.mu.RUnlock()

	if _, ok := r.Header["Hx-Request"]; !ok {

		clone, err := s.templates.ExtendedTemplates("features")
		if err != nil {
			slog.Error("Error when extending templates", "page", "features")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.topicSourcesMu.RLock()
		source, ok := s.registery.Get(s.topicSources[topic])
		if !ok {
			slog.Error("Did not find client in registery from topic sources", "client id", s.topicSources[topic])
		}
		clone.RenderPage(w, "feature_detail", map[string]interface{}{
			"Feature":       client.Meta().Capabilities[topic],
			"FeatureSource": source.Meta(),
			"Features":      s.topicSources,
			"Subscriptions": subs,
		})
		s.topicSourcesMu.RUnlock()
	} else {
		clone, err := s.templates.ExtendedTemplates("feature_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "feature_detail")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.topicSourcesMu.RLock()
		source, ok := s.registery.Get(s.topicSources[topic])
		if !ok {
			slog.Error("Did not find client in registery from topic sources", "client id", s.topicSources[topic])
		}
		clone.Render(w, "content", map[string]interface{}{
			"Feature":       client.Meta().Capabilities[topic],
			"FeatureSource": source.Meta(),
			"Subscriptions": subs,
		})
		s.topicSourcesMu.RUnlock()
	}
}

func (s *GohabServer) HandleTransports(w http.ResponseWriter, r *http.Request) {
	s.templates.RenderPage(w, "transports", map[string]interface{}{
		"Transports": s.transports,
	})
}

func (s *GohabServer) HandleTransportDetail(w http.ResponseWriter, r *http.Request) {
	index := chi.URLParam(r, "i")
	i, err := strconv.Atoi(index)
	if err != nil {
		slog.Warn("Error converting transport index into int", "index", index)
		http.Error(w, "Invalid transport index: "+index, http.StatusBadRequest)
		return
	}
	if _, ok := r.Header["Hx-Request"]; !ok {

		clone, err := s.templates.ExtendedTemplates("transports")
		if err != nil {
			slog.Error("Error when extending templates", "page", "transports")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.topicSourcesMu.RLock()
		clone.RenderPage(w, "transport_detail", map[string]interface{}{
			"Transports": s.transports,
			"Transport":  s.transports[i].Meta(),
		})
		s.topicSourcesMu.RUnlock()
	} else {
		clone, err := s.templates.ExtendedTemplates("transport_detail")
		if err != nil {
			slog.Error("Error when extending templates", "page", "transport_detail")
			http.Error(w, "Template extension error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		clone.Render(w, "content", map[string]interface{}{
			"Transport": s.transports[i].Meta(),
		})
	}
}

func (s *GohabServer) HandleHome(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/devices", http.StatusMovedPermanently)
}
