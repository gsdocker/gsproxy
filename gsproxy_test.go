package gsproxy

import (
	"bytes"
	"com/gsrpc/test"
	"math/big"
	"testing"
	"time"

	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	"github.com/gsrpc/gorpc/net"
)

var log = gslogger.Get("acceptor")

type _MockProxy struct {
	server Server
}

func (mock *_MockProxy) OpenProxy(context Context) {
}

func (mock *_MockProxy) CloseProxy(context Context) {
}

func (mock *_MockProxy) CreateServer(server Server) error {
	log.D("create server (%p)", server)
	mock.server = server
	return nil
}

func (mock *_MockProxy) CloseServer(server Server) {
}

func (mock *_MockProxy) CreateDevice(device Device) error {

	log.D("create new device(%s)", device.ID().ID)

	device.Bind(0, mock.server)

	return nil
}

func (mock *_MockProxy) CloseDevice(device Device) {
	log.D("close device(%s)", device.ID().ID)
}

type _MockTunnelClient struct {
	gslogger.Log
	device gorpc.Device
}

func tunnelClientHandler() gorpc.Handler {
	return &_MockTunnelClient{
		Log: gslogger.Get("agent-client-tunnel"),
	}
}

func (handler *_MockTunnelClient) OpenHandler(context gorpc.Context) error {
	return nil
}

func (handler *_MockTunnelClient) CloseHandler(context gorpc.Context) {

}

func (handler *_MockTunnelClient) HandleWrite(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	tunnel, err := gorpc.ReadTunnel(bytes.NewBuffer(message.Content))

	if err != nil {
		handler.E("backward tunnel(%s) message -- failed\n%s", tunnel.ID, err)
		return nil, err
	}

	handler.device = *tunnel.ID

	return tunnel.Message, nil
}

func (handler *_MockTunnelClient) HandleRead(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	tunnel := gorpc.NewTunnel()

	tunnel.ID = &handler.device

	tunnel.Message = message

	var buff bytes.Buffer

	err := gorpc.WriteTunnel(&buff, tunnel)

	if err != nil {
		return nil, err
	}

	message.Code = gorpc.CodeTunnel

	message.Content = buff.Bytes()

	return message, nil
}

func (handler *_MockTunnelClient) HandleError(context gorpc.Context, err error) error {

	return err
}

type mockRESTful struct {
	content map[string][]byte
}

func (mock *mockRESTful) Post(name string, content []byte) (err error) {
	mock.content[name] = content
	return nil
}

func (mock *mockRESTful) Get(name string) (retval []byte, err error) {

	val, ok := mock.content[name]

	if ok {
		return val, nil
	}

	return nil, test.NewNotFound()
}

var dispatcher = test.MakeRESTful(0, &mockRESTful{
	content: make(map[string][]byte),
})

func createDevice(name string) (gorpc.Sink, *net.TCPClient) {
	G, _ := new(big.Int).SetString("6849211231874234332173554215962568648211715948614349192108760170867674332076420634857278025209099493881977517436387566623834457627945222750416199306671083", 0)

	P, _ := new(big.Int).SetString("13196520348498300509170571968898643110806720751219744788129636326922565480984492185368038375211941297871289403061486510064429072584259746910423138674192557", 0)

	clientSink := gorpc.NewSink(name, time.Second*5, 8, 10)

	return clientSink, net.NewTCPClient(
		"127.0.0.1:13512",
		gorpc.BuildPipeline().Handler(
			"log-client",
			func() gorpc.Handler {
				return gorpc.LoggerHandler()
			},
		).Handler(
			"dh-client",
			func() gorpc.Handler {
				return net.NewCryptoClient(gorpc.NewDevice(), G, P)
			},
		).Handler(
			"sink-client",
			func() gorpc.Handler {
				return clientSink
			},
		),
	).Connect(time.Second * 1)
}

func createServer(name string) (gorpc.Sink, *net.TCPClient) {
	clientSink := gorpc.NewSink(name, time.Second*5, 8, 10)

	clientSink.Register(dispatcher)

	return clientSink, net.NewTCPClient(
		"127.0.0.1:15827",
		gorpc.BuildPipeline().Handler(
			"log-server",
			func() gorpc.Handler {
				return gorpc.LoggerHandler()
			},
		).Handler(
			"tunnel-client",
			func() gorpc.Handler {
				return tunnelClientHandler()
			},
		).Handler(
			"sink-server",
			func() gorpc.Handler {
				return clientSink
			},
		),
	).Connect(time.Second * 1)
}

func TestAgent(t *testing.T) {

	BuildProxy(&_MockProxy{}).Run("gsproxy-test")

	_, server := createServer("gsrproxy-server-test")

	<-time.After(time.Second)

	clientSink, client := createDevice("gsproxy-device-test")

	api := test.BindRESTful(0, clientSink)

	client.D("call Post")

	err := api.Post("nil", nil)

	if err != nil {
		t.Fatal(err)
	}

	client.D("call Post -- success")

	content, err := api.Get("nil")

	if err != nil {
		t.Fatal(err)
	}

	if content != nil {
		t.Fatal("rpc test error")
	}

	api.Post("hello", []byte("hello world"))

	content, err = api.Get("hello")

	if err != nil {
		t.Fatal(err)
	}

	if string(content) != "hello world" {
		t.Fatal("rpc test error")
	}

	_, err = api.Get("hello2")

	if err == nil {
		t.Fatal("expect (*test.NotFound)exception")
	}

	if _, ok := err.(*test.NotFound); !ok {
		t.Fatal("expect (*test.NotFound)exception")
	}

	<-time.After(time.Second)

	server.Close()

	client.Close()

	<-time.After(time.Second)
}
