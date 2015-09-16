package gsproxy

import (
	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	"github.com/gsrpc/gorpc/handler"
)

type _Client struct {
	gslogger.Log                // mixin Log APIs
	name         string         // client name
	pipeline     gorpc.Pipeline // Mixin pipeline
	context      *_Proxy        // proxy belongs to
	device       *gorpc.Device  // device name
}

func (proxy *_Proxy) newClientHandler() gorpc.Handler {

	return &_Client{
		Log:     gslogger.Get("gsproxy-client"),
		context: proxy,
	}
}

func (client *_Client) Register(context gorpc.Context) error {

	return nil
}

func (client *_Client) Active(context gorpc.Context) error {

	dh, _ := context.Pipeline().Handler(dhHandler)

	device := dh.(handler.CryptoServer).GetDevice()

	client.pipeline = context.Pipeline()

	client.device = device

	client.context.addClient(client)

	return nil
}

func (client *_Client) Unregister(context gorpc.Context) {

}

func (client *_Client) Inactive(context gorpc.Context) {
	if client.device != nil {
		client.context.removeClient(client)
	}
}

func (client *_Client) MessageReceived(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	return message, nil
}
func (client *_Client) MessageSending(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	return message, nil
}

func (client *_Client) Panic(context gorpc.Context, err error) {

}

func (client *_Client) Close() {
	client.pipeline.Close()
}

func (client *_Client) AddService(dispatcher gorpc.Dispatcher) {
	client.pipeline.AddService(dispatcher)
}

func (client *_Client) removeService(dispatcher gorpc.Dispatcher) {
	client.pipeline.RemoveService(dispatcher)
}

func (client *_Client) SendMessage(message *gorpc.Message) error {
	return client.pipeline.SendMessage(message)
}

func (client *_Client) String() string {
	return client.name
}

func (client *_Client) Name() string {
	return client.name
}

func (client *_Client) Device() *gorpc.Device {
	return client.device
}

func (client *_Client) Bind(id uint16, server Server) {
	handler, _ := client.pipeline.Handler(transProxyHandler)
	handler.(*_TransProxyHandler).bind(id, server)
}
func (client *_Client) Unbind(id uint16) {
	handler, _ := client.pipeline.Handler(transProxyHandler)
	handler.(*_TransProxyHandler).unbind(id)
}
