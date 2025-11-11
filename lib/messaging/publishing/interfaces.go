package publishing

// MessagePublisher interface for sending messages
type MessagePublisher interface {
	PublishMessage(route string, body any) error
	PublishRawMessage(route string, body []byte) error
	PublishTextMessage(route string, text string) error
}
