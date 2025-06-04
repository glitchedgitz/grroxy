package grrhttp

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/glitchedgitz/grroxy-db/utils"
)

func DecompressResponse(reader io.Reader, contentEncoding string) (io.Reader, error) {
	switch strings.ToLower(contentEncoding) {
	case "gzip":
		return gzip.NewReader(reader)
	case "br":
		return brotli.NewReader(reader), nil
	default:
		return reader, nil
	}
}

func DumpResponse(resp *http.Response) string {

	var size = 0
	var err error
	var bodyReader io.Reader

	contentLength := resp.Header.Get("Content-Length")

	// Check if we should download the resource or not
	if contentLength != "" {
		size, err = strconv.Atoi(contentLength)
		utils.CheckErr("[DumpResponse]", err)
	}

	if size > 0 {
		resp.ContentLength = int64(size)

		bodyReader, err = DecompressResponse(resp.Body, resp.Header.Get("Content-Encoding"))
		utils.CheckErr("", err)
	} else {
		bodyReader = resp.Body
	}

	// defer bodyReader.Close()

	// var bodyReader io.ReadCloser
	// if resp.Header.Get("Content-Encoding") == "gzip" {
	// 	bodyReader, err = gzip.NewReader(resp.Body)
	// 	if err != nil {
	// 		// fallback to raw data
	// 		bodyReader = resp.Body
	// 	}
	// } else {
	// 	bodyReader = resp.Body
	// }

	bf := bufio.NewReader(bodyReader)
	var cl int64
	respbody, err := io.ReadAll(bf)
	utils.CheckErr("", err)
	cl = int64(len(respbody))

	finalResp := fmt.Sprintf("%s %s\n", resp.Proto, resp.Status)

	for header, value := range resp.Header {
		if strings.Contains(strings.ToLower(header), "Content-Encoding") {
			value = []string{fmt.Sprintf("%d", cl)}
		}
		for _, val := range value {
			finalResp += fmt.Sprintf("%s: %s\n", header, val)
		}
	}

	finalResp += "\n" + string(respbody)

	return finalResp
}

// 	bf := bufio.NewReader(bodyReader)

// 	finalResp := fmt.Sprintf("%s %s\n", resp.Proto, resp.Status)

// 	for header, value := range resp.Header {
// 		finalResp += fmt.Sprintf("%s: %s\n", header, value)
// 	}

// 	if respbody, err := io.ReadAll(bf); err == nil {
// 		resp.ContentLength = int64(len(respbody))
// 		resp.Body = io.NopCloser(bytes.NewReader(respbody))
// 		finalResp, err := httputils.DumpResponse(resp, true)
// 		utils.CheckErr("[DumpResponse]", err)
// 		return string(finalResp)
// 	} else {
// 		log.Println("Error reading response body:", err)
// 		return ""
// 	}
// }
