package transport

type Message struct {
	Topic   string `json:"topic"`
	Payload []byte `json:"payload"`
	Sender  string `json:"sender"`
}

type Transport interface {
	Start() error
	OnMessage(func(Message))
	Shutdown() error
}
