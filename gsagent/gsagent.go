package gsagent

import (
	"sync"
	"time"

	"github.com/gsdocker/gsconfig"
	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	"github.com/gsrpc/gorpc/tcp"
)

// Agent device agent
type Agent interface {
	gorpc.Channel
	// ID agent id
	ID() *gorpc.Device

	Close()
	// AddService
	AddService(dispatcher gorpc.Dispatcher)
	// RemoveService remove agent servie
	RemoveService(dispatcher gorpc.Dispatcher)
}

// System .
type System interface {
	Register(context Context) error
	Unregister(context Context)
	BindAgent(agent Agent) error
	UnbindAgent(agent Agent)
	AgentServices() []*gorpc.NamedService
	AddTunnel(name string, pipeline gorpc.Pipeline)
	RemoveTunnel(name string, pipeline gorpc.Pipeline)
}

// Context .
type Context interface {
	Name() string
	// Close agent system
	Close()
	// Connect
	Connect(name string, raddr string) (tcp.Client, error)
}

// AgentBuilder .
type AgentBuilder struct {
	system     System        // agent system
	cachedsize int           // send cached
	timeout    time.Duration // rpc call timeout
	reconnect  time.Duration // reconnect to gsproxy service delay time duration
}

// BuildAgent .
func BuildAgent(system System) *AgentBuilder {
	return &AgentBuilder{
		system:     system,
		cachedsize: gsconfig.Int("gsagent.rpc.sendQ", 1024),
		timeout:    gsconfig.Seconds("gsagent.rpc.timeout", 5),
		reconnect:  gsconfig.Seconds("gsagent.reconnect.delay", 5),
	}
}

// SendQ set send Q
func (builder *AgentBuilder) SendQ(cached int) *AgentBuilder {
	builder.cachedsize = cached
	return builder
}

// Timeout set rpc timeout
func (builder *AgentBuilder) Timeout(duration time.Duration) *AgentBuilder {

	builder.timeout = duration

	return builder
}

// Reconnect set reconnect delay duration
func (builder *AgentBuilder) Reconnect(duration time.Duration) *AgentBuilder {

	builder.reconnect = duration

	return builder
}

type _System struct {
	gslogger.Log                           // mixin log APIs
	sync.RWMutex                           // mutex
	name         string                    // name
	timeout      time.Duration             // rpc call timeout
	system       System                    // agent system
	eventLoop    gorpc.EventLoop           // event loop
	reconnect    time.Duration             // reconnect to gsproxy service delay time duration
	cachedsize   int                       // send cached
	tunnels      map[string]*_TunnelClient // register tunnel
}

// Build .
func (builder *AgentBuilder) Build(name string, eventLoop gorpc.EventLoop) Context {
	context := &_System{
		name:       name,
		system:     builder.system,
		eventLoop:  eventLoop,
		timeout:    builder.timeout,
		reconnect:  builder.reconnect,
		cachedsize: builder.cachedsize,
		tunnels:    make(map[string]*_TunnelClient),
	}

	return context
}

func (system *_System) Close() {

}

func (system *_System) Name() string {
	return system.name
}

func (system *_System) addTunnel(name string, tunnel *_TunnelClient, pipeline gorpc.Pipeline) {
	system.Lock()
	defer system.Unlock()

	system.tunnels[name] = tunnel

	system.system.AddTunnel(name, pipeline)
}

func (system *_System) removeTunnel(name string, tunnel *_TunnelClient, pipeline gorpc.Pipeline) {
	system.Lock()
	defer system.Unlock()

	if target, ok := system.tunnels[name]; ok && target == tunnel {
		delete(system.tunnels, name)
		system.system.RemoveTunnel(name, pipeline)
	}

}

// Connect
func (system *_System) Connect(name string, raddr string) (tcp.Client, error) {

	return tcp.BuildClient(
		gorpc.BuildPipeline(system.eventLoop).Handler(
			"profile",
			gorpc.ProfileHandler,
		).Handler(
			"tunnel-client",
			func() gorpc.Handler {
				return system.newTunnelClient(name)
			},
		).Timeout(system.timeout),
	).Cached(system.cachedsize).Remote(raddr).Reconnect(system.reconnect).Connect(name)
}
