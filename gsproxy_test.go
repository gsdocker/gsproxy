package gsproxy

import (
	"runtime"
	"testing"
	"time"

	"./gsagent"
	"github.com/gsrpc/gorpc"
)

type _MockProxy struct {
	Services chan []*gorpc.NamedService
}

func (mock *_MockProxy) Register(context Context) error {
	return nil
}

func (mock *_MockProxy) Unregister(context Context) {

}

func (mock *_MockProxy) BindServices(context Context, server Server, services []*gorpc.NamedService) error {
	mock.Services <- services
	return nil
}

func (mock *_MockProxy) UnbindServices(context Context, server Server) {

}

func (mock *_MockProxy) AddClient(context Context, client Client) error {
	return nil
}

func (mock *_MockProxy) RemoveClient(context Context, client Client) {

}

type _MockAgent struct {
	Tunnel chan gorpc.Pipeline
}

func (mock *_MockAgent) Register(context gsagent.Context) error {
	return nil
}

func (mock *_MockAgent) Unregister(context gsagent.Context) {

}

func (mock *_MockAgent) BindAgent(agent gsagent.Agent) error {
	return nil
}

func (mock *_MockAgent) UnbindAgent(agent gsagent.Agent) {

}

func (mock *_MockAgent) AgentServices() []*gorpc.NamedService {
	return nil
}

func (mock *_MockAgent) AddTunnel(name string, pipeline gorpc.Pipeline) {
	mock.Tunnel <- pipeline
}

func (mock *_MockAgent) RemoveTunnel(name string, pipeline gorpc.Pipeline) {

}

var mockAgent = &_MockAgent{
	Tunnel: make(chan gorpc.Pipeline),
}

var mockProxy = &_MockProxy{
	Services: make(chan []*gorpc.NamedService),
}

var eventLoop = gorpc.NewEventLoop(uint32(runtime.NumCPU()), 2048, 500*time.Millisecond)

var agentSystem = gsagent.BuildAgent(mockAgent).Build("gsagent-test", eventLoop)

var gsProxy = BuildProxy(mockProxy).Build("gsproxy-test", eventLoop)

func TestConnect(t *testing.T) {

	_, err := agentSystem.Connect("gsagent-gsproxy", "localhost:15827")

	if err != nil {
		t.Fatal(err)
	}

	<-mockAgent.Tunnel

	<-mockProxy.Services
}
