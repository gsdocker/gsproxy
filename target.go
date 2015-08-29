package gsproxy

import (
	"sync"
	"time"

	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	"github.com/gsrpc/gorpc/net"
)

type _ServerTarget struct {
	gslogger.Log               // mixin log
	gorpc.Sink                 // mixin sink
	proxy        *_Proxy       // proxy
	context      gorpc.Context // context
}

func (proxy *_Proxy) newServerTarget(name string, timeout time.Duration, cached int, processors int) gorpc.Sink {
	return &_ServerTarget{
		Log:   gslogger.Get(name),
		proxy: proxy,
		Sink:  gorpc.NewSink(name, timeout, cached, processors),
	}
}

func (server *_ServerTarget) OpenHandler(context gorpc.Context) error {

	err := server.Sink.OpenHandler(context)

	if err != nil {
		return err
	}

	server.context = context

	return server.proxy.proxy.CreateServer(server)
}

func (server *_ServerTarget) SendMessage(message *gorpc.Message) error {
	return server.Sink.SendMessage(message)
}

func (server *_ServerTarget) Close() {
	server.context.Close()
}

type _DeviceTarget struct {
	sync.RWMutex                   // mixin Mutex
	gslogger.Log                   // mixin log
	gorpc.Sink                     // mixin sink
	proxy        *_Proxy           // proxy
	dhHandler    string            // dhhandler name
	device       *gorpc.Device     // device
	context      gorpc.Context     // context
	servers      map[uint16]Server // servers
}

func (proxy *_Proxy) newDeviceTarget(name string, dhHandler string, timeout time.Duration, cached int, processors int) gorpc.Sink {
	return &_DeviceTarget{
		Log:       gslogger.Get(name),
		Sink:      gorpc.NewSink(name, timeout, cached, processors),
		proxy:     proxy,
		dhHandler: dhHandler,
		servers:   make(map[uint16]Server),
	}
}

func (device *_DeviceTarget) OpenHandler(context gorpc.Context) error {

	device.D("open device handler")

	err := device.Sink.OpenHandler(context)

	if err != nil {
		return err
	}

	handler, _ := context.GetHandler(device.dhHandler)

	device.device = handler.(net.CryptoServer).GetDevice()

	device.context = context

	err = device.proxy.proxy.CreateDevice(device)

	if err == nil {
		device.D("open device handler -- success")

		device.proxy.addDevice(device)
	}

	return err
}

func (device *_DeviceTarget) CloseHandler(context gorpc.Context) {

	device.proxy.proxy.CloseDevice(device)

	device.proxy.removeDevice(device)

	device.D("close device handler -- success")
}

func (device *_DeviceTarget) Close() {
	device.context.Close()
}

func (device *_DeviceTarget) ID() gorpc.Device {
	return *device.device
}

func (device *_DeviceTarget) Bind(id uint16, server Server) {
	device.Lock()
	defer device.Unlock()

	device.D("bind server(%d:%p)", id, server)

	device.servers[id] = server
}
func (device *_DeviceTarget) Unbind(id uint16) {
	device.Lock()
	defer device.Unlock()

	delete(device.servers, id)
}

// Bound get bound server
func (device *_DeviceTarget) bound(id uint16) (server Server, ok bool) {
	device.RLock()
	defer device.RUnlock()

	server, ok = device.servers[id]

	return
}
