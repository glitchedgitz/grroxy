package endpoints

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/grrhttp"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/tomnomnom/rawhttp"
	"golang.org/x/net/http2"
)

func SendHTTPRawRequest(data rawhttp.RawRequest) (string, error) {
	// Connect to the server
	var host = data.Hostname
	var port = data.Port
	var rawRequest = data.Request

	log.Println("Port: ", port)

	if port == "" {
		if data.TLS {
			port = "443"
		} else {
			port = "80"
		}
	}

	var addr = host + ":" + port
	log.Println("Addr: ", addr)

	var conn net.Conn
	var err error
	if data.TLS {
		conn, err = tls.DialWithDialer(&net.Dialer{
			Timeout: data.Timeout,
		}, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
		})
	} else {
		conn, err = net.DialTimeout("tcp", addr, data.Timeout)
	}
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// Send the raw request
	_, err = fmt.Fprintf(conn, "%s\r\n\r\n", rawRequest)
	if err != nil {
		return "", fmt.Errorf("failed to write the request: %w", err)
	}

	// Read the response
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return "", fmt.Errorf("failed to read the response: %w", err)
	}
	defer resp.Body.Close()

	return grrhttp.DumpResponse(resp), nil
}

func SendHTTP2RawRequest(data rawhttp.RawRequest) (string, error) {
	// Connect to the server
	var host = data.Hostname
	var port = data.Port
	var rawRequest = data.Request

	r := bufio.NewReader(strings.NewReader(rawRequest))
	s, err := r.ReadString('\n')
	if err != nil {
		log.Printf("could not read request: %s", err)
	}
	parts := strings.Split(s, " ")
	if len(parts) < 3 {
		log.Printf("malformed request supplied")
	}
	// Set the request Method
	Method := parts[0]
	Path := parts[1]
	Headers := make(map[string]string)

	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)

		if err != nil || line == "" {
			break
		}

		p := strings.SplitN(line, ":", 2)
		if len(p) != 2 {
			continue
		}

		if strings.EqualFold(p[0], "content-length") {
			continue
		}

		Headers[strings.TrimSpace(p[0])] = strings.TrimSpace(p[1])
	}

	log.Println("Port: ", port)

	if port == "" {
		if data.TLS {
			port = "443"
		} else {
			port = "80"
		}
	}

	var addr = host + ":" + port
	log.Println("Addr: ", addr)

	// Convert the raw HTTP request to a HTTP/2 request
	var buf bytes.Buffer
	b, err := io.ReadAll(r)
	base.CheckErr("", err)
	buf.WriteString(string(b))

	// Configure the HTTP/2 Transport
	http2.ConfigureTransports(&http.Transport{
		ForceAttemptHTTP2: true,
		// Proxy:               proxyURL,
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 500,
		MaxConnsPerHost:     500,
		DialContext: (&net.Dialer{
			Timeout: time.Duration(time.Duration(10) * time.Second),
		}).DialContext,
		TLSHandshakeTimeout: time.Duration(time.Duration(10) * time.Second),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS10,
			Renegotiation:      tls.RenegotiateOnceAsClient,
			// ServerName:         conf.SNI,
		},
	})

	// Send the raw HTTP/2 request
	req, err := http.NewRequest(Method, Path, &buf)
	req.Host = addr
	req.URL.Host = addr
	req.URL.Scheme = "https"

	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	// var client *http.Client
	for header, value := range Headers {
		req.Header.Set(header, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP/2 request: %w", err)
	}
	defer resp.Body.Close()

	return grrhttp.DumpResponse(resp), nil
}

func (pocketbaseDB *DatabaseAPI) SendRawRequest(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/sendrawrequest",
		Handler: func(c echo.Context) error {
			var data map[string]interface{}
			if err := c.Bind(&data); err != nil {
				return err
			}

			host := data["host"].(string)
			host = strings.TrimPrefix(host, "http://")
			host = strings.TrimPrefix(host, "https://")

			request := data["req"].(string)
			// replace \n with \r\n

			// request = strings.ReplaceAll(request, "\n", "\r\n") + "\r\n\r\n"

			log.Println("RawRequest TLS: ", data["tls"].(bool))
			log.Println("RawRequest Hostname: ", data["host"].(string))
			log.Println("RawRequest Port: ", data["port"].(string))
			log.Println("RawRequest Timeout: ", time.Duration(data["timeout"].(float64))*time.Second)
			log.Println("RawRequest Request: ", request)

			mappedData := rawhttp.RawRequest{
				TLS:      data["tls"].(bool),
				Hostname: host,
				Port:     data["port"].(string),
				Request:  request,
				Timeout:  time.Duration(data["timeout"].(float64)) * time.Second,
			}

			// respString, err := sendRawRequest2(mappedData)
			var respString = ""
			var err error

			log.Println("httpversion: ", data["httpversion"])

			if data["httpversion"].(float64) == 1 {
				respString, err = SendHTTPRawRequest(mappedData)
			} else {
				respString, err = SendHTTP2RawRequest(mappedData)
			}

			if err != nil {
				return err
			}

			response := map[string]interface{}{
				"resp": respString,
			}

			return c.JSON(http.StatusOK, response)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(pocketbaseDB.App),
		},
	})
	return nil
}
