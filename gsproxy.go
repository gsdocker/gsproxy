package gsproxy

import (
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/gsdocker/gsconfig"
	"github.com/gsdocker/gslogger"
	"github.com/gsrpc/gorpc"
	"github.com/gsrpc/gorpc/net"
)

// Target Proxied Endpoint
type Target interface {
	Close()
	// Mixin MessageChannel
	gorpc.MessageChannel
	// Unreigster dispatcher
	Unregister(dispatcher gorpc.Dispatcher)
	// Register dispatcher
	Register(dispatcher gorpc.Dispatcher)
}

// Server .
type Server Target

// Device .
type Device interface {
	Target
	// ID get device id
	ID() gorpc.Device
	// Bind bind service by id
	Bind(id uint16, server Server)
	// Unbind unbind service by id
	Unbind(id uint16)
}

// Context .
type Context interface {
}

// Proxy .
type Proxy interface {
	OpenProxy(context Context)
	CreateServer(server Server) error
	CloseServer(server Server)
	CreateDevice(device Device) error
	CloseDevice(device Device)
}

// ProxyBuilder gsproxy builder
type ProxyBuilder struct {
	frontend      string            // frontend _Server listen addr
	backend       string            // backend _Server listen addr
	timeout       time.Duration     // rpc timeout
	dhkeyResolver net.DHKeyResolver // dhkey resolver
	proxy         Proxy             // proxy implement
}

// BuildProxy create new proxy builder
func BuildProxy(proxy Proxy) *ProxyBuilder {
	gStr := gsconfig.String("agent.dhkey.G", "6849211231874234332173554215962568648211715948614349192108760170867674332076420634857278025209099493881977517436387566623834457627945222750416199306671083")

	pStr := gsconfig.String("agent.dhkey.P", "13196520348498300509170571968898643110806720751219744788129636326922565480984492185368038375211941297871289403061486510064429072584259746910423138674192557")

	G, _ := new(big.Int).SetString(gStr, 0)

	P, _ := new(big.Int).SetString(pStr, 0)

	return &ProxyBuilder{

		frontend: gsconfig.String("agent.frontend.laddr", ":13512"),

		backend: gsconfig.String("agent.backend.laddr", ":15827"),

		timeout: gsconfig.Seconds("agent.rpc.timeout", 5),

		dhkeyResolver: net.DHKeyResolve(func(device *gorpc.Device) (*net.DHKey, error) {
			return net.NewDHKey(G, P), nil
		}),

		proxy: proxy,
	}
}

// AddrF set frontend listen address
func (builder *ProxyBuilder) AddrF(laddr string) *ProxyBuilder {
	builder.frontend = laddr
	return builder
}

// AddrB set backend listen address
func (builder *ProxyBuilder) AddrB(laddr string) *ProxyBuilder {
	builder.backend = laddr
	return builder
}

// DHKeyResolver set frontend dhkey resolver
func (builder *ProxyBuilder) DHKeyResolver(dhkeyResolver net.DHKeyResolver) *ProxyBuilder {
	builder.dhkeyResolver = dhkeyResolver
	return builder
}

// _Proxy .
type _Proxy struct {
	sync.RWMutex                           // mutex
	gslogger.Log                           // mixin log APIs
	frontend     *net.TCPServer            // frontend
	backend      *net.TCPServer            // backend
	proxy        Proxy                     // proxy implement
	devices      map[string]*_DeviceTarget // handle agent clients
	name         string                    //proxy name
}

// Run create new agent _Server instance and run it
func (builder *ProxyBuilder) Run(name string) {

	proxy := &_Proxy{
		Log:     gslogger.Get(name),
		proxy:   builder.proxy,
		devices: make(map[string]*_DeviceTarget),
		name:    name,
	}

	proxy.frontend = net.NewTCPServer(
		gorpc.BuildPipeline().Handler(
			fmt.Sprintf("%s-log-fe", name),
			func() gorpc.Handler {
				return gorpc.LoggerHandler()
			},
		).Handler(
			fmt.Sprintf("%s-dh-fe", name),
			func() gorpc.Handler {
				return net.NewCryptoServer(builder.dhkeyResolver)
			},
		).Handler(
			fmt.Sprintf("%s-proxy-fe", name),
			func() gorpc.Handler {
				return transProxyHandle(proxy, fmt.Sprintf("%s-sink-fe", name))
			},
		).Handler(
			fmt.Sprintf("%s-sink-fe", name),
			func() gorpc.Handler {
				return proxy.newDeviceTarget(fmt.Sprintf("%s-sink-fe", name), fmt.Sprintf("%s-dh-fe", name), builder.timeout, 1024, runtime.NumCPU())
			},
		),
	)

	proxy.backend = net.NewTCPServer(
		gorpc.BuildPipeline().Handler(
			fmt.Sprintf("%s-log-be", name),
			func() gorpc.Handler {
				return gorpc.LoggerHandler()
			},
		).Handler(
			fmt.Sprintf("%s-be", name),
			func() gorpc.Handler {
				return TunnelServerHandler(proxy)
			},
		).Handler(
			fmt.Sprintf("%s-sink-be", name),
			func() gorpc.Handler {
				return proxy.newServerTarget(fmt.Sprintf("%s-sink-be", name), builder.timeout, 1024, runtime.NumCPU())
			},
		),
	)

	go func() {
		if err := proxy.backend.Listen(builder.backend); err != nil {
			proxy.E("start agent backend error :%s", err)
		}
	}()

	go func() {
		if err := proxy.frontend.Listen(builder.frontend); err != nil {
			proxy.E("start agent frontend error :%s", err)
		}
	}()
}

func (proxy *_Proxy) device(id *gorpc.Device) (target *_DeviceTarget, ok bool) {
	proxy.RLock()
	defer proxy.RUnlock()

	target, ok = proxy.devices[id.ID]

	return
}

func (proxy *_Proxy) addDevice(target *_DeviceTarget) {
	proxy.Lock()
	defer proxy.Unlock()

	if old, ok := proxy.devices[target.device.ID]; ok {
		go old.Close()
	}

	proxy.devices[target.device.ID] = target

}

func (proxy *_Proxy) removeDevice(target *_DeviceTarget) {
	proxy.Lock()
	defer proxy.Unlock()

	delete(proxy.devices, target.device.ID)
}
