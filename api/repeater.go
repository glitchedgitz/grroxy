package api

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
	"github.com/pocketbase/pocketbase/models"
	"github.com/tomnomnom/rawhttp"
	"golang.org/x/net/http2"
)

func SendHTTPRawRequest(data rawhttp.RawRequest) (string, string, error) {
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
	var timeBefore time.Time
	var timeAfter time.Time
	var timeTaken string

	if data.TLS {
		timeBefore = time.Now()
		conn, err = tls.DialWithDialer(&net.Dialer{
			Timeout: data.Timeout,
		}, "tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
		})
		timeAfter = time.Now()
	} else {
		timeBefore = time.Now()
		conn, err = net.DialTimeout("tcp", addr, data.Timeout)
		timeAfter = time.Now()
	}
	if err != nil {
		return "", "", err
	}

	timeTaken = base.CalculateTime(timeBefore, timeAfter)

	defer conn.Close()

	// Send the raw request
	_, err = fmt.Fprintf(conn, "%s\r\n\r\n", rawRequest)
	if err != nil {
		return "", "", fmt.Errorf("failed to write the request: %w", err)
	}

	// Read the response
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to read the response: %w", err)
	}
	defer resp.Body.Close()

	return grrhttp.DumpResponse(resp), timeTaken, nil
}

func SendHTTP2RawRequest(data rawhttp.RawRequest) (string, string, error) {
	// Connect to the server
	var host = data.Hostname
	var port = data.Port
	var rawRequest = data.Request
	var timeBefore time.Time
	var timeAfter time.Time
	var timeTaken string

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
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	// var client *http.Client
	for header, value := range Headers {
		req.Header.Set(header, value)
	}

	timeBefore = time.Now()
	resp, err := http.DefaultClient.Do(req)
	timeAfter = time.Now()

	timeTaken = base.CalculateTime(timeBefore, timeAfter)

	if err != nil {
		return "", "", fmt.Errorf("failed to send HTTP/2 request: %w", err)
	}
	defer resp.Body.Close()

	return grrhttp.DumpResponse(resp), timeTaken, nil
}

func (backend *Backend) SendRawRequest(e *core.ServeEvent) error {

	e.Router.AddRoute(echo.Route{
		Method: http.MethodPost,
		Path:   "/api/sendrawrequest",
		Handler: func(c echo.Context) error {
			admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
			recordd, _ := c.Get(apis.ContextAuthRecordKey).(*models.Record)

			isGuest := admin == nil && recordd == nil

			if isGuest {
				return c.String(http.StatusForbidden, "")
			}

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

			// Get the current time

			// Create another time instance, for example, 1 hour ahead

			// Calculate the time difference
			var timeTaken = ""

			if data["httpversion"].(float64) == 1 {
				respString, timeTaken, err = SendHTTPRawRequest(mappedData)
			} else {
				respString, timeTaken, err = SendHTTP2RawRequest(mappedData)
			}

			if err != nil {
				return err
			}

			response := map[string]interface{}{
				"resp": respString,
				"time": timeTaken,
			}

			return c.JSON(http.StatusOK, response)
		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
