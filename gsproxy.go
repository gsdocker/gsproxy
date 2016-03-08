package gsproxy

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/gsdocker/gsconfig"
	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	"github.com/gsrpc/gorpc/handler"
)

var (
	dhHandler         = "gsproxy-dh"
	transProxyHandler = "gsproxy-trans"
	tunnelHandler     = "gsproxy-tunnel"
)

// Context .
type Context interface {
	String() string
	// Close close proxy
	Close()
	// get frontend acceptor
	Acceptor() *gorpc.Acceptor
}

// Server server
type Server gorpc.Pipeline

// Client proxy client
type Client interface {
	AddService(dispatcher gorpc.Dispatcher)

	RemoveService(dispatcher gorpc.Dispatcher)
	// TransproxyBind bind transproxy service by id
	TransproxyBind(id uint16, server Server)
	// Unbind unbind transproxy service by id
	TransproxyUnbind(id uint16)
	// Device get device name
	Device() *gorpc.Device
}

// Proxy .
type Proxy interface {
	// Register register current proxy server
	Register(context Context) error
	// Unregister unregister proxy
	Unregister(context Context)
	// BindServices add server to proxy session
	BindServices(context Context, server Server, services []*gorpc.NamedService) error
	// UnbindServices remote server from proxy session
	UnbindServices(context Context, server Server)
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
	gStr := gsconfig.String("gsproxy.dhkey.G", "6849211231874234332173554215962568648211715948614349192108760170867674332076420634857278025209099493881977517436387566623834457627945222750416199306671083")

	pStr := gsconfig.String("gsproxy.dhkey.P", "13196520348498300509170571968898643110806720751219744788129636326922565480984492185368038375211941297871289403061486510064429072584259746910423138674192557")

	G, _ := new(big.Int).SetString(gStr, 0)

	P, _ := new(big.Int).SetString(pStr, 0)

	return &ProxyBuilder{

		laddrF: gsconfig.String("gsproxy.frontend.laddr", ":13512"),

		laddrE: gsconfig.String("gsproxy.backend.laddr", ":15827"),

		timeout: gsconfig.Seconds("gsproxy.rpc.timeout", 5),

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

// Heartbeat .
func (builder *ProxyBuilder) Heartbeat(timeout time.Duration) *ProxyBuilder {
	builder.timeout = timeout
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
	frontend     *gorpc.Acceptor     // frontend
	backend      *gorpc.Acceptor     // backend
	proxy        Proxy               // proxy implement
	clients      map[string]*_Client // handle agent clients
	idgen        byte                // tunnel id gen
	tunnels      map[byte]byte       // tunnels
}

// Build .
func (builder *ProxyBuilder) Build(name string) Context {

	proxy := &_Proxy{
		Log:     gslogger.Get("gsproxy"),
		proxy:   builder.proxy,
		clients: make(map[string]*_Client),
		name:    name,
		tunnels: make(map[byte]byte),
	}

	proxy.frontend = gorpc.NewAcceptor(
		fmt.Sprintf("%s.frontend", name),
		gorpc.BuildPipeline(time.Millisecond*10).Handler(
			"gsproxy-profile",
			gorpc.ProfileHandler,
		).Handler(
			"gsproxy-dh",
			func() gorpc.Handler {
				return handler.NewCryptoServer(builder.dhkeyResolver)
			},
		).Handler(
			"gsproxy-hb",
			func() gorpc.Handler {
				return handler.NewHeartbeatHandler(builder.timeout)
			},
		).Handler(
			transProxyHandler,
			proxy.newTransProxyHandler,
		).Handler(
			"gsproxy-client",
			proxy.newClientHandler,
		),
	)

	proxy.backend = gorpc.NewAcceptor(
		fmt.Sprintf("%s.backend", name),
		gorpc.BuildPipeline(time.Millisecond*10).Handler(
			tunnelHandler,
			proxy.newTunnelServer,
		),
	)

	go func() {
		if err := gorpc.TCPListen(proxy.backend, builder.laddrE); err != nil {
			proxy.E("start agent backend error :%s", err)
		}
	}()

	go func() {
		if err := gorpc.TCPListen(proxy.frontend, builder.laddrF); err != nil {
			proxy.E("start agent frontend error :%s", err)
		}
	}()

	return proxy
}

func (proxy *_Proxy) Acceptor() *gorpc.Acceptor {
	return proxy.frontend
}

func (proxy *_Proxy) String() string {
	return proxy.name
}

func (proxy *_Proxy) Close() {

}

func (proxy *_Proxy) removeTunnelID(id byte) {

	proxy.Lock()
	defer proxy.Unlock()

	delete(proxy.tunnels, id)
}

func (proxy *_Proxy) tunnelID() byte {

	proxy.Lock()
	defer proxy.Unlock()

	for {
		proxy.idgen++

		if _, ok := proxy.tunnels[proxy.idgen]; !ok && proxy.idgen != 0 {
			proxy.tunnels[proxy.idgen] = 0

			return proxy.idgen
		}
	}
}

func (proxy *_Proxy) client(device *gorpc.Device) (*_Client, bool) {
	proxy.RLock()
	defer proxy.RUnlock()

	client, ok := proxy.clients[device.String()]

	return client, ok
}

func (proxy *_Proxy) addClient(client *_Client) {

	proxy.Lock()
	defer proxy.Unlock()

	if client, ok := proxy.clients[client.device.String()]; ok {

		proxy.proxy.RemoveClient(proxy, client)

		client.Close()
	}

	proxy.clients[client.device.String()] = client

	proxy.proxy.AddClient(proxy, client)
}

func (proxy *_Proxy) removeClient(client *_Client) {

	proxy.Lock()
	defer proxy.Unlock()

	device := client.device

	if old, ok := proxy.clients[device.String()]; ok && client == old {
		proxy.proxy.RemoveClient(proxy, client)
	}
}
