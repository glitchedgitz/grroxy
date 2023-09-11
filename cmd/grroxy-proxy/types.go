package main

import (
	"context"

	"github.com/armon/go-socks5"
	"github.com/elazarl/goproxy"
	"github.com/glitchedgitz/grroxy-db/certs"
	"github.com/glitchedgitz/grroxy-db/save"
	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/haxii/fastproxy/bufiopool"
	"github.com/haxii/fastproxy/superproxy"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	rbtransport "github.com/projectdiscovery/roundrobin/transport"
	"github.com/projectdiscovery/tinydns"
)

type Options struct {
	DumpRequest                 bool
	DumpResponse                bool
	Silent                      bool
	Verbosity                   bool
	CertCacheSize               int
	Directory                   string
	ListenAddrHTTP              string
	ListenAddrSocks5            string
	OutputDirectory             string
	RequestDSL                  string
	ResponseDSL                 string
	UpstreamHTTPProxies         []string
	UpstreamSock5Proxies        []string
	ListenDNSAddr               string
	DNSMapping                  string
	DNSFallbackResolver         string
	RequestMatchReplaceDSL      string
	ResponseMatchReplaceDSL     string
	OnConnectHTTPCallback       OnConnectFunc
	OnConnectHTTPSCallback      OnConnectFunc
	OnRequestCallback           OnRequestFunc
	OnResponseCallback          OnResponseFunc
	Deny                        []string
	Allow                       []string
	UpstreamProxyRequestsNumber int
	// Elastic                     *elastic.Options
	// Kafka                       *kafka.Options
	Intercept bool
	Waiting   bool
	Ctx       context.Context
}

type Proxy struct {
	Dialer       *fastdialer.Dialer
	options      *Options
	save         *save.Logger
	certs        *certs.Manager
	httpproxy    *goproxy.ProxyHttpServer
	socks5proxy  *socks5.Server
	socks5tunnel *superproxy.SuperProxy
	bufioPool    *bufiopool.Pool
	tinydns      *tinydns.TinyDNS
	rbhttp       *rbtransport.RoundTransport
	rbsocks5     *rbtransport.RoundTransport
	grroxydb     *sdk.Client
}
