package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/armon/go-socks5"
	"github.com/elazarl/goproxy"
	certs "github.com/glitchedgitz/grroxy-db/certs"
	save "github.com/glitchedgitz/grroxy-db/save"
	"github.com/glitchedgitz/grroxy-db/schemas"
	"github.com/glitchedgitz/grroxy-db/sdk"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/haxii/fastproxy/bufiopool"
	"github.com/haxii/fastproxy/superproxy"
	"github.com/pocketbase/pocketbase/models"
	pbTypes "github.com/pocketbase/pocketbase/tools/types"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	rbtransport "github.com/projectdiscovery/roundrobin/transport"
	"github.com/projectdiscovery/tinydns"
	"golang.org/x/net/proxy"
)

type OnRequestFunc func(*http.Request, *goproxy.ProxyCtx) (*http.Request, *http.Response)
type OnResponseFunc func(*http.Response, *goproxy.ProxyCtx) *http.Response
type OnConnectFunc func(string, *goproxy.ProxyCtx) (*goproxy.ConnectAction, string)

func (p *Proxy) Startup(ctx context.Context) {
	p.options.Ctx = ctx
}

func (p *Proxy) OnConnectHTTP(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
	ctx.UserData = types.UserData{Host: host}
	return goproxy.HTTPMitmConnect, host
}

func (p *Proxy) OnConnectHTTPS(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
	ctx.UserData = types.UserData{Host: host}
	return goproxy.MitmConnect, host
}

func (p *Proxy) RunProxy() error {

	if p.tinydns != nil {
		go p.tinydns.Run()
	}

	// http proxy
	if p.httpproxy != nil {
		if len(p.options.UpstreamHTTPProxies) > 0 {
			p.httpproxy.Tr = &http.Transport{Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse(p.rbhttp.Next())
			}, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
			p.httpproxy.ConnectDial = nil
		} else if len(p.options.UpstreamSock5Proxies) > 0 {
			// for each socks5 proxy create a dialer
			socks5Dialers := make(map[string]proxy.Dialer)
			for _, socks5proxy := range p.options.UpstreamSock5Proxies {
				dialer, err := proxy.SOCKS5("tcp", socks5proxy, nil, proxy.Direct)
				if err != nil {
					return err
				}
				socks5Dialers[socks5proxy] = dialer
			}
			p.httpproxy.Tr = &http.Transport{Dial: func(network, addr string) (net.Conn, error) {
				// lookup next dialer
				socks5Proxy := p.rbsocks5.Next()
				socks5Dialer := socks5Dialers[socks5Proxy]
				// use it to perform the request
				return socks5Dialer.Dial(network, addr)
			}, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
			p.httpproxy.ConnectDial = nil
		} else {
			p.httpproxy.Tr.DialContext = p.Dialer.Dial
		}
		onConnectHTTP := p.OnConnectHTTP
		if p.options.OnConnectHTTPCallback != nil {
			onConnectHTTP = p.options.OnConnectHTTPCallback
		}
		onConnectHTTPS := p.OnConnectHTTPS
		if p.options.OnConnectHTTPSCallback != nil {
			onConnectHTTPS = p.options.OnConnectHTTPSCallback
		}
		onRequest := p.OnRequest
		if p.options.OnRequestCallback != nil {
			onRequest = p.options.OnRequestCallback
		}
		onResponse := p.OnResponse
		if p.options.OnResponseCallback != nil {
			onResponse = p.options.OnResponseCallback
		}

		// Doubt: What about other ports
		p.httpproxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*:80$"))).HandleConnectFunc(onConnectHTTP)
		p.httpproxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*:443$"))).HandleConnectFunc(onConnectHTTPS)
		// catch all
		p.httpproxy.OnRequest().HandleConnectFunc(onConnectHTTPS)
		p.httpproxy.OnRequest().DoFunc(onRequest)
		p.httpproxy.OnResponse().DoFunc(onResponse)

		// Serve the certificate when the user makes requests to /grroxy
		p.httpproxy.OnRequest(goproxy.DstHostIs("grroxy")).DoFunc(
			func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
				if r.URL.Path != "/cacert.crt" {
					return r, goproxy.NewResponse(r, "text/plain", 404, "Invalid path given")
				}

				_, ca := p.certs.GetCA()
				reader := bytes.NewReader(ca)

				header := http.Header{}
				header.Set("Content-Type", "application/pkix-cert")
				resp := &http.Response{
					Request:          r,
					TransferEncoding: r.TransferEncoding,
					Header:           header,
					StatusCode:       200,
					Status:           http.StatusText(200),
					ContentLength:    int64(reader.Len()),
					Body:             io.NopCloser(reader),
				}
				return r, resp
			},
		)

		// p.httpproxy.OnResponse(goproxy.DstHostIs("grroxy")).DoFunc(
		// 	func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		// 		return resp
		// 	},
		// )

		go http.ListenAndServe(p.options.ListenAddrHTTP, p.httpproxy) // nolint
	}

	// socks5 proxy
	if p.socks5proxy != nil {
		if p.httpproxy != nil {
			httpProxyIP, httpProxyPort, err := net.SplitHostPort(p.options.ListenAddrHTTP)
			if err != nil {
				return err
			}
			httpProxyPortUint, err := strconv.ParseUint(httpProxyPort, 10, 16)
			if err != nil {
				return err
			}
			p.socks5tunnel, err = superproxy.NewSuperProxy(httpProxyIP, uint16(httpProxyPortUint), superproxy.ProxyTypeHTTP, "", "", "")
			if err != nil {
				return err
			}
			p.bufioPool = bufiopool.New(4096, 4096)
		}

		return p.socks5proxy.ListenAndServe("tcp", p.options.ListenAddrSocks5)
	}

	return nil
}

func (p *Proxy) Stop() {

}

func NewProxy(options *Options) (*Proxy, error) {

	os.MkdirAll(options.Directory, os.ModePerm) //nolint

	var grroxydb = sdk.NewClient(
		"http://127.0.0.1:8090",
		sdk.WithAdminEmailPassword("new@example.com", "1234567890"))

	certs, err := certs.New(&certs.Options{
		CacheSize: options.CertCacheSize,
		Directory: options.Directory,
	})
	if err != nil {
		return nil, err
	}

	var httpproxy *goproxy.ProxyHttpServer
	if options.ListenAddrHTTP != "" {
		httpproxy = goproxy.NewProxyHttpServer()
		// if options.Silent {
		// 	httpproxy.Logger = log.New(ioutil.Discard, "", log.Ltime|log.Lshortfile)
		// } else if options.Verbose {
		// 	httpproxy.Verbose = true
		// } else {
		// 	httpproxy.Verbose = false
		// }
	}

	ca, _ := certs.GetCA()
	goproxy.GoproxyCa = ca
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: certs.TLSConfigFromCA()}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: certs.TLSConfigFromCA()}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: certs.TLSConfigFromCA()}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: certs.TLSConfigFromCA()}

	logger := save.NewLogger(&save.OptionsLogger{
		// OutputFolder: path.Join(conf.DatabaseDirectory, options.OutputDirectory),
	})

	var tdns *tinydns.TinyDNS

	fastdialerOptions := fastdialer.DefaultOptions
	fastdialerOptions.EnableFallback = true
	fastdialerOptions.Deny = options.Deny
	fastdialerOptions.Allow = options.Allow
	if options.ListenDNSAddr != "" {
		dnsmapping := make(map[string]string)
		for _, record := range strings.Split(options.DNSMapping, ",") {
			data := strings.Split(record, ":")
			if len(data) != 2 {
				continue
			}
			dnsmapping[data[0]] = data[1]
		}
		tdns = tinydns.NewTinyDNS(&tinydns.OptionsTinyDNS{
			ListenAddress:       options.ListenDNSAddr,
			Net:                 "udp",
			FallbackDNSResolver: options.DNSFallbackResolver,
			DomainToAddress:     dnsmapping,
		})
		fastdialerOptions.BaseResolvers = []string{"127.0.0.1" + options.ListenDNSAddr}
	}
	dialer, err := fastdialer.NewDialer(fastdialerOptions)
	if err != nil {
		return nil, err
	}

	var rbhttp, rbsocks5 *rbtransport.RoundTransport
	if len(options.UpstreamHTTPProxies) > 0 {
		rbhttp, err = rbtransport.NewWithOptions(options.UpstreamProxyRequestsNumber, options.UpstreamHTTPProxies...)
		if err != nil {
			return nil, err
		}
	}

	if len(options.UpstreamSock5Proxies) > 0 {
		rbsocks5, err = rbtransport.NewWithOptions(options.UpstreamProxyRequestsNumber, options.UpstreamSock5Proxies...)
		if err != nil {
			return nil, err
		}
	}

	proxy := &Proxy{
		httpproxy: httpproxy,
		certs:     certs,
		save:      logger,
		options:   options,
		Dialer:    dialer,
		tinydns:   tdns,
		rbhttp:    rbhttp,
		rbsocks5:  rbsocks5,
		grroxydb:  grroxydb,
	}

	proxy.grroxydb.CreateCollection(models.Collection{
		Name:       "tmp_intercept",
		Type:       models.CollectionTypeBase,
		ListRule:   pbTypes.Pointer(""),
		ViewRule:   pbTypes.Pointer(""),
		CreateRule: pbTypes.Pointer(""),
		UpdateRule: pbTypes.Pointer(""),
		DeleteRule: nil,
		Schema:     schemas.Intercept,
	})

	proxy.DBCreate("_ui", map[string]string{
		"id":        "___INTERCEPT___",
		"unique_id": "___INTERCEPT___",
		"data":      `{"filters": [],"filterstring":"","sort": "created"}`,
	})

	var socks5proxy *socks5.Server
	if options.ListenAddrSocks5 != "" {
		socks5Config := &socks5.Config{
			Dial: proxy.httpTunnelDialer,
		}
		if options.Silent {
			socks5Config.Logger = log.New(io.Discard, "", log.Ltime|log.Lshortfile)
		}
		socks5proxy, err = socks5.New(socks5Config)
		if err != nil {
			return nil, err
		}
	}

	proxy.socks5proxy = socks5proxy
	go proxy.InterceptManager()
	go proxy.FiltersManager()
	go proxy.RateLimitManager()

	return proxy, nil
}

func (p *Proxy) httpTunnelDialer(ctx context.Context, network, addr string) (net.Conn, error) {
	return p.socks5tunnel.MakeTunnel(nil, nil, p.bufioPool, addr)
}
