package gsproxy

import (
	"math/big"
	"sync"
	"time"

	"github.com/gsdocker/gsconfig"
	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	"github.com/gsrpc/gorpc/handler"
	"github.com/gsrpc/gorpc/tcp"
)

var (
	dhHandler         = "gsproxy-dh"
	transProxyHandler = "gsproxy-trans"
)

// Context .
type Context interface {
	String() string
	// Close close proxy
	Close()
}

// Server server
type Server gorpc.Pipeline

// Client proxy client
type Client interface {
	gorpc.Pipeline
	// Bind bind service by id
	Bind(id uint16, server Server)
	// Unbind unbind service by id
	Unbind(id uint16)
	// Device get device name
	Device() *gorpc.Device
}

// Proxy .
type Proxy interface {
	// Register register current proxy server
	Register(context Context)
	// Unregister unregister proxy
	Unregister(context Context)
	// AddServer add server to proxy session
	AddServer(context Context, server Server) error
	// RemoveServer remote server from proxy session
	RemoveServer(context Context, server Server)
	// AddClient add client to proxy session
	AddClient(context Context, client Client) error
	// RemoveClient remote client from proxy session
	RemoveClient(context Context, client Client)
}

// ProxyBuilder gsproxy builder
type ProxyBuilder struct {
	laddrF        string                // frontend tcp listen address
	laddrE        string                // backend tcp listen address
	timeout       time.Duration         // rpc timeout
	dhkeyResolver handler.DHKeyResolver // dhkey resolver
	proxy         Proxy                 // proxy provider
}

// BuildProxy create new proxy builder
func BuildProxy(proxy Proxy) *ProxyBuilder {
	gStr := gsconfig.String("agent.dhkey.G", "6849211231874234332173554215962568648211715948614349192108760170867674332076420634857278025209099493881977517436387566623834457627945222750416199306671083")

	pStr := gsconfig.String("agent.dhkey.P", "13196520348498300509170571968898643110806720751219744788129636326922565480984492185368038375211941297871289403061486510064429072584259746910423138674192557")

	G, _ := new(big.Int).SetString(gStr, 0)

	P, _ := new(big.Int).SetString(pStr, 0)

	return &ProxyBuilder{

		laddrF: gsconfig.String("agent.frontend.laddr", ":13512"),

		laddrE: gsconfig.String("agent.backend.laddr", ":15827"),

		timeout: gsconfig.Seconds("agent.rpc.timeout", 5),

		dhkeyResolver: handler.DHKeyResolve(func(device *gorpc.Device) (*handler.DHKey, error) {
			return handler.NewDHKey(G, P), nil
		}),

		proxy: proxy,
	}
}

// AddrF set frontend listen address
func (builder *ProxyBuilder) AddrF(laddr string) *ProxyBuilder {
	builder.laddrF = laddr
	return builder
}

// AddrB set backend listen address
func (builder *ProxyBuilder) AddrB(laddr string) *ProxyBuilder {
	builder.laddrE = laddr
	return builder
}

// DHKeyResolver set frontend dhkey resolver
func (builder *ProxyBuilder) DHKeyResolver(dhkeyResolver handler.DHKeyResolver) *ProxyBuilder {
	builder.dhkeyResolver = dhkeyResolver
	return builder
}

type _Proxy struct {
	sync.RWMutex                     // mutex
	gslogger.Log                     // mixin log APIs
	name         string              //proxy name
	frontend     *tcp.Server         // frontend
	backend      *tcp.Server         // backend
	proxy        Proxy               // proxy implement
	clients      map[string]*_Client // handle agent clients
}

// Build .
func (builder *ProxyBuilder) Build(name string, executor gorpc.EventLoop) Context {

	proxy := &_Proxy{
		Log:     gslogger.Get("gsproxy"),
		proxy:   builder.proxy,
		clients: make(map[string]*_Client),
		name:    name,
	}

	proxy.frontend = tcp.NewServer(
		gorpc.BuildPipeline(executor).Handler(
			"gsproxy-dh",
			func() gorpc.Handler {
				return handler.NewCryptoServer(builder.dhkeyResolver)
			},
		),
	).EvtNewPipeline(
		tcp.EvtNewPipeline(func(pipeline gorpc.Pipeline) {
			proxy.addClient(pipeline)
		}),
	).EvtClosePipeline(
		tcp.EvtClosePipeline(func(pipeline gorpc.Pipeline) {
			proxy.removeClient(pipeline)
		}),
	)

	proxy.backend = tcp.NewServer(
		gorpc.BuildPipeline(executor).Handler("tunnel", proxy.newTunnelServer),
	).EvtNewPipeline(
		tcp.EvtNewPipeline(func(pipeline gorpc.Pipeline) {

			proxy.proxy.AddServer(proxy, Server(pipeline))
		}),
	).EvtClosePipeline(
		tcp.EvtClosePipeline(func(pipeline gorpc.Pipeline) {
			proxy.proxy.RemoveServer(proxy, Server(pipeline))
		}),
	)

	go func() {
		if err := proxy.backend.Listen(builder.laddrE); err != nil {
			proxy.E("start agent backend error :%s", err)
		}
	}()

	go func() {
		if err := proxy.frontend.Listen(builder.laddrF); err != nil {
			proxy.E("start agent frontend error :%s", err)
		}
	}()

	return proxy
}

func (proxy *_Proxy) String() string {
	return proxy.name
}

func (proxy *_Proxy) Close() {

}

func (proxy *_Proxy) client(device *gorpc.Device) (*_Client, bool) {
	proxy.RLock()
	defer proxy.RUnlock()

	client, ok := proxy.clients[device.String()]

	return client, ok
}
