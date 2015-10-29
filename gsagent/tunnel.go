package gsagent

import (
	"bytes"
	"sync"
	"time"

	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
)

type _Agent struct {
	gorpc.Sink
	handler *_TunnelClient
	id      *gorpc.Device
}

func newAgent(handler *_TunnelClient, device *gorpc.Device) (*_Agent, error) {
	context := &_Agent{
		handler: handler,
		id:      device,
	}

	context.Sink = gorpc.NewSink(device.String(), handler.eventLoop, context, handler.timeout)

	var err error

	err = handler.system.system.BindAgent(context)

	if err != nil {
		return nil, err
	}

	return context, nil
}

func (agent *_Agent) SendMessage(message *gorpc.Message) error {
	return agent.handler.SendMessage(agent.id, message)
}

func (agent *_Agent) Close() {
	agent.handler.system.system.UnbindAgent(agent)
}

// ID agent id
func (agent *_Agent) ID() *gorpc.Device {
	return agent.id
}

// _TunnelClient .
type _TunnelClient struct {
	sync.RWMutex                    // mutex
	gslogger.Log                    // mixin log APIs
	name         string             // tunnel name
	system       *_System           // system
	context      gorpc.Context      // context
	agents       map[string]*_Agent // agents
	eventLoop    gorpc.EventLoop    // event loop
	timeout      time.Duration      // rpc timeout
}

func (system *_System) newTunnelClient(name string) gorpc.Handler {
	return &_TunnelClient{
		Log:       gslogger.Get("gsagent-tunnel"),
		name:      name,
		system:    system,
		eventLoop: system.eventLoop,
		timeout:   system.timeout,
	}
}

func (handler *_TunnelClient) Register(context gorpc.Context) error {
	handler.context = context
	return nil
}

func (handler *_TunnelClient) Active(context gorpc.Context) error {

	handler.agents = make(map[string]*_Agent)

	handler.system.addTunnel(handler.name, handler, context.Pipeline())

	// send TunnelWhoAmI

	whoAmI := gorpc.NewTunnelWhoAmI()

	whoAmI.Services = handler.system.system.AgentServices()

	var buff bytes.Buffer

	gorpc.WriteTunnelWhoAmI(&buff, whoAmI)

	message := gorpc.NewMessage()

	message.Code = gorpc.CodeTunnelWhoAmI

	message.Content = buff.Bytes()

	context.Send(message)

	return nil
}

func (handler *_TunnelClient) Unregister(context gorpc.Context) {
}

func (handler *_TunnelClient) Inactive(context gorpc.Context) {
	for _, agent := range handler.agents {
		agent.Close()
	}

	handler.system.removeTunnel(handler.name, handler, context.Pipeline())
}

func (handler *_TunnelClient) agent(device *gorpc.Device) (*_Agent, error) {

	handler.Lock()
	defer handler.Unlock()

	if agent, ok := handler.agents[device.String()]; ok {
		return agent, nil
	}

	agent, err := newAgent(handler, device)

	if err != nil {
		return nil, err
	}

	handler.agents[device.String()] = agent

	return agent, nil
}

func (handler *_TunnelClient) SendMessage(device *gorpc.Device, message *gorpc.Message) error {

	tunnel := gorpc.NewTunnel()

	tunnel.ID = device

	tunnel.Message = message

	var buff bytes.Buffer

	err := gorpc.WriteTunnel(&buff, tunnel)

	if err != nil {
		return err
	}

	message.Code = gorpc.CodeTunnel

	message.Content = buff.Bytes()

	handler.context.Send(message)
	return nil
}

func (handler *_TunnelClient) Close() {

}

func (handler *_TunnelClient) MessageReceived(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	if message.Code != gorpc.CodeTunnel {

		return message, nil
	}

	handler.V("handle tunnel message")

	tunnel, err := gorpc.ReadTunnel(bytes.NewBuffer(message.Content))

	if err != nil {
		handler.E("backward tunnel(%s) message -- failed\n%s", tunnel.ID, err)
		return nil, err
	}

	handler.I("dispatch tunnel message to %s", tunnel.ID)

	agent, err := handler.agent(tunnel.ID)

	if err != nil {
		handler.E("dispatch tunnel(%s) message -- failed\n%s", tunnel.ID, err)
		return nil, nil
	}

	go agent.MessageReceived(tunnel.Message)

	return nil, nil
}

func (handler *_TunnelClient) MessageSending(context gorpc.Context, message *gorpc.Message) (*gorpc.Message, error) {

	return message, nil
}

func (handler *_TunnelClient) Panic(context gorpc.Context, err error) {

}
