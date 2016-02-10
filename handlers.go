package gsproxy

import (
	"bytes"
	"sync"

	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	gorpcHandler "github.com/gsrpc/gorpc/handler"
)

type _TunnelServerHandler struct {
	gslogger.Log         // mixin log APIs
	proxy        *_Proxy // proxy
	id           byte    // agnet id
}

func (proxy *_Proxy) newTunnelServer() gorpc.Handler {
	return &_TunnelServerHandler{
		Log:   gslogger.Get("agent-server-tunnel"),
		proxy: proxy,
		id:    proxy.tunnelID(),
	}
}

func (handler *_TunnelServerHandler) Register(context gorpc.Context) error {
	return nil
}

func (handler *_TunnelServerHandler) Active(context gorpc.Context) error {
	return gorpc.ErrSkip
}

func (handler *_TunnelServerHandler) Unregister(context gorpc.Context) {

}

func (handler *_TunnelServerHandler) Inactive(context gorpc.Context) {
	go handler.proxy.proxy.UnbindServices(handler.proxy, context.Pipeline())
}

func (handler *_TunnelServerHandler) CloseHandler(context gorpc.Context) {

}

func (handler *_TunnelServerHandler) MessageReceived(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	if message.Code == gorpc.CodeTunnelWhoAmI {

		handler.I("tunnel handshake ......")

		whoAmI, err := gorpc.ReadTunnelWhoAmI(bytes.NewBuffer(message.Content))

		if err != nil {
			return nil, err
		}

		handler.proxy.proxy.BindServices(handler.proxy, context.Pipeline(), whoAmI.Services)

		context.FireActive()

		return nil, nil
	}

	if message.Code != gorpc.CodeTunnel {
		return message, nil
	}

	handler.V("backward tunnel message")

	tunnel, err := gorpc.ReadTunnel(bytes.NewBuffer(message.Content))

	if err != nil {
		handler.E("backward tunnel(%s) message -- failed\n%s", tunnel.ID, err)
		return nil, err
	}

	if device, ok := handler.proxy.client(tunnel.ID); ok {

		tunnel.Message.Agent = handler.id

		err := device.SendMessage(tunnel.Message)

		if err == nil {
			handler.V("backward tunnel message -- success")
			return nil, nil
		}

		return nil, err
	}

	handler.E("backward tunnel(%s) message -- failed,device not found", tunnel.ID)

	return nil, nil
}

func (handler *_TunnelServerHandler) MessageSending(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	return message, nil
}

func (handler *_TunnelServerHandler) Panic(context gorpc.Context, err error) {

}

func (handler *_TunnelServerHandler) ID() byte {
	return handler.id
}

type _TransProxyHandler struct {
	gslogger.Log                   // mixin log APIs
	sync.RWMutex                   // mixin rw locker
	proxy        *_Proxy           // proxy
	client       *_Client          // client
	device       *gorpc.Device     // devices
	servers      map[uint16]Server // bound servers
	tunnels      map[byte]Server   // bound servers
}

func (proxy *_Proxy) newTransProxyHandler() gorpc.Handler {
	return &_TransProxyHandler{
		Log:     gslogger.Get("trans-proxy"),
		proxy:   proxy,
		servers: make(map[uint16]Server),
		tunnels: make(map[byte]Server),
	}
}

func (handler *_TransProxyHandler) bind(id uint16, server Server) {
	handler.Lock()
	defer handler.Unlock()

	tunnel, _ := server.Handler(tunnelHandler)

	handler.servers[id] = server

	handler.tunnels[tunnel.(*_TunnelServerHandler).ID()] = server
}

func (handler *_TransProxyHandler) unbind(id uint16) {
	handler.Lock()
	defer handler.Unlock()

	delete(handler.servers, id)
}

func (handler *_TransProxyHandler) Register(context gorpc.Context) error {
	return nil
}

func (handler *_TransProxyHandler) Active(context gorpc.Context) error {

	dh, _ := context.Pipeline().Handler(dhHandler)

	handler.device = dh.(gorpcHandler.CryptoServer).GetDevice()

	return nil
}

func (handler *_TransProxyHandler) Unregister(context gorpc.Context) {

}

func (handler *_TransProxyHandler) Inactive(context gorpc.Context) {

}

func (handler *_TransProxyHandler) forward(server Server, message *gorpc.Message) error {
	handler.V("forward tunnel(%s) message", handler.device)

	tunnel := gorpc.NewTunnel()

	tunnel.ID = handler.device

	tunnel.Message = message

	var buff bytes.Buffer

	err := gorpc.WriteTunnel(&buff, tunnel)

	if err != nil {
		return err
	}

	message.Code = gorpc.CodeTunnel

	message.Content = buff.Bytes()

	err = server.SendMessage(message)

	if err == nil {
		handler.V("forward tunnel(%s) message(%p) -- success", handler.device, message)
	} else {
		handler.E("forward tunnel(%s) message -- failed\n%s", handler.device, err)
	}

	return err
}

func (handler *_TransProxyHandler) tunnel(agent byte) (Server, bool) {

	handler.RLock()
	defer handler.RUnlock()

	server, ok := handler.tunnels[agent]

	return server, ok
}

func (handler *_TransProxyHandler) transproxy(service uint16) (Server, bool) {

	handler.RLock()
	defer handler.RUnlock()

	server, ok := handler.servers[service]

	return server, ok
}

func (handler *_TransProxyHandler) MessageReceived(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	if message.Code == gorpc.CodeResponse {

		if server, ok := handler.tunnel(message.Agent); ok {

			err := handler.forward(server, message)

			if err != nil {
				context.Close()
			}

			return nil, err
		}

		return message, nil

	}

	if message.Code != gorpc.CodeRequest {

		return message, nil
	}

	request, err := gorpc.ReadRequest(bytes.NewBuffer(message.Content))

	if err != nil {
		handler.E("[%s] unmarshal request error\n%s", handler.proxy.name, err)
		return nil, err
	}

	service := request.Service

	if transproxy, ok := handler.transproxy(service); ok {

		handler.V("forward tunnel(%s) message", handler.device)

		tunnel := gorpc.NewTunnel()

		tunnel.ID = handler.device

		tunnel.Message = message

		var buff bytes.Buffer

		err := gorpc.WriteTunnel(&buff, tunnel)

		if err != nil {
			return nil, err
		}

		message.Code = gorpc.CodeTunnel

		message.Content = buff.Bytes()

		err = transproxy.SendMessage(message)

		if err != nil {
			context.Close()
			handler.V("forward tunnel(%s) message(%p) -- failed\n%s", handler.device, message, err)
			return nil, err
		}

		handler.V("forward tunnel(%s) message(%p) -- success", handler.device, message)

		return nil, err
	}

	return message, nil
}

func (handler *_TransProxyHandler) MessageSending(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	return message, nil
}

func (handler *_TransProxyHandler) Panic(context gorpc.Context, err error) {

}
