package gsproxy

import (
	"bytes"

	"github.com/gsdocker/gserrors"
	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
)

type _TransProxyHandler struct {
	gslogger.Log                 // mixin log APIs
	proxy         *_Proxy        // proxy
	deviceHandler string         // dhhandler
	target        *_DeviceTarget // target
}

func transProxyHandle(proxy *_Proxy, deviceHandler string) gorpc.Handler {
	return &_TransProxyHandler{
		Log:           gslogger.Get("trans-proxy"),
		proxy:         proxy,
		deviceHandler: deviceHandler,
	}
}

func (handler *_TransProxyHandler) OpenHandler(context gorpc.Context) error {

	handler.D("open request handler")

	device, ok := context.GetHandler(handler.deviceHandler)

	if !ok {
		return gserrors.Newf(nil, "inner error,dh handler not found")
	}

	handler.target = device.(*_DeviceTarget)

	handler.D("open request handler --  success")

	return nil
}

func (handler *_TransProxyHandler) CloseHandler(context gorpc.Context) {

}

func (handler *_TransProxyHandler) HandleWrite(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	request, err := gorpc.ReadRequest(bytes.NewBuffer(message.Content))

	if err != nil {
		handler.E("[%s] unmarshal request error\n%s", handler.proxy.name, err)
		return nil, err
	}

	if server, ok := handler.target.bound(request.Service); ok {

		handler.D("forward tunnel(%s) message", handler.target.device)

		tunnel := gorpc.NewTunnel()

		tunnel.ID = handler.target.device

		tunnel.Message = message

		var buff bytes.Buffer

		err := gorpc.WriteTunnel(&buff, tunnel)

		if err != nil {
			return nil, err
		}

		message.Code = gorpc.CodeTunnel

		message.Content = buff.Bytes()

		err = server.SendMessage(message)

		if err == nil {
			handler.D("forward tunnel(%s) message(%p) -- success", handler.target.device, message)
		} else {
			handler.E("forward tunnel(%s) message -- failed\n%s", handler.target.device, err)
		}

		return nil, err
	}

	return message, nil
}

func (handler *_TransProxyHandler) HandleRead(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	return message, nil
}

func (handler *_TransProxyHandler) HandleError(context gorpc.Context, err error) error {

	return err
}
