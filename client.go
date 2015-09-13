package gsproxy

import (
	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	"github.com/gsrpc/gorpc/handler"
)

type _Client struct {
	gslogger.Log                 // mixin Log APIs
	name           string        // client name
	gorpc.Pipeline               // Mixin pipeline
	context        *_Proxy       // proxy belongs to
	device         *gorpc.Device // device name
}

func (proxy *_Proxy) addClient(pipeline gorpc.Pipeline) {

	dh, _ := pipeline.Handler(dhHandler)

	device := dh.(handler.CryptoServer).GetDevice()

	proxy.Lock()
	defer proxy.Unlock()

	if client, ok := proxy.clients[device.String()]; ok {

		proxy.proxy.RemoveClient(proxy, client)

		client.Close()
	}

	client := &_Client{
		Pipeline: pipeline,
		context:  proxy,
		name:     device.String(),
		device:   device,
	}

	proxy.clients[device.String()] = client

	proxy.proxy.AddClient(proxy, client)
}

func (proxy *_Proxy) removeClient(pipeline gorpc.Pipeline) {
	dh, _ := pipeline.Handler(dhHandler)

	device := dh.(handler.CryptoServer).GetDevice()

	proxy.Lock()
	defer proxy.Unlock()

	if client, ok := proxy.clients[device.String()]; ok {

		proxy.proxy.RemoveClient(proxy, client)

		client.Close()
	}
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
	handler, _ := client.Pipeline.Handler(transProxyHandler)
	handler.(*_TransProxyHandler).bind(id, server)
}
func (client *_Client) Unbind(id uint16) {
	handler, _ := client.Pipeline.Handler(transProxyHandler)
	handler.(*_TransProxyHandler).unbind(id)
}
