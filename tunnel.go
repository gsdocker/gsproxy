package gsproxy

import (
	"bytes"

	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
)

type _TunnelServerHandler struct {
	gslogger.Log         // mixin log APIs
	proxy        *_Proxy // proxy
}

// TunnelServerHandler create new agent reverse proxy handler
func TunnelServerHandler(proxy *_Proxy) gorpc.Handler {
	return &_TunnelServerHandler{
		Log:   gslogger.Get("agent-server-tunnel"),
		proxy: proxy,
	}
}

func (handler *_TunnelServerHandler) OpenHandler(context gorpc.Context) error {
	return nil
}

func (handler *_TunnelServerHandler) CloseHandler(context gorpc.Context) {

}

func (handler *_TunnelServerHandler) HandleWrite(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	if message.Code != gorpc.CodeTunnel {
		return message, nil
	}

	handler.V("backward tunnel message")

	tunnel, err := gorpc.ReadTunnel(bytes.NewBuffer(message.Content))

	if err != nil {
		handler.E("backward tunnel(%s) message -- failed\n%s", tunnel.ID, err)
		return nil, err
	}

	if device, ok := handler.proxy.device(tunnel.ID); ok {
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

func (handler *_TunnelServerHandler) HandleRead(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	return message, nil
}

func (handler *_TunnelServerHandler) HandleError(context gorpc.Context, err error) error {

	return err
}
