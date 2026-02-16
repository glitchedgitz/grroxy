package rawproxy

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const divider = "-------------------|-------------------\n"

// captureData holds data for async capture operations
type captureData struct {
	reqDump    []byte
	respDump   []byte
	request    *http.Request
	requestID  string
	outputDir  string
	reqCounter *atomic.Uint64
}

// Channel for async capture writing
var captureQueue = make(chan captureData, 100)

func init() {
	// Start capture writer goroutine
	// go captureWriter()
}

func captureWriter() {
	for data := range captureQueue {
		if path, err := writeCaptureToDir(data.reqDump, data.respDump, data.request, data.outputDir, data.reqCounter); err != nil {
			log.Printf("[ERROR] requestID=%s url=%s err=%v", data.requestID, data.request.URL.String(), err)
		} else {
			log.Printf("[SAVED] requestID=%s url=%s file=%s", data.requestID, data.request.URL.String(), path)
		}
	}
}

func asyncCapture(reqDump, respDump []byte, r *http.Request, requestID string, config *Config) {
	asyncCaptureToDir(reqDump, respDump, r, requestID, config.OutputDir, config.ReqCounter)
}

func asyncWebSocketCapture(reqDump, respDump []byte, r *http.Request, requestID string, config *Config) {
	asyncCaptureToDir(reqDump, respDump, r, requestID, config.WebSocketDir, config.ReqCounter)
}

func asyncCaptureToDir(reqDump, respDump []byte, r *http.Request, requestID string, dir string, reqCounter *atomic.Uint64) {
	select {
	case captureQueue <- captureData{reqDump: reqDump, respDump: respDump, request: r, requestID: requestID, outputDir: dir, reqCounter: reqCounter}:
		// Queued successfully
	default:
		// Queue full, log error but don't block
		log.Printf("[WARN] requestID=%s capture queue full, dropping capture for %s", requestID, r.URL.String())
	}
}

func writeCaptureToDir(reqDump []byte, respDump []byte, r *http.Request, dir string, reqCounter *atomic.Uint64) (string, error) {
	ts := time.Now().UTC().Format("20060102-150405.000000000")
	idx := reqCounter.Add(1)
	safeHost := strings.ReplaceAll(r.Host, string(filepath.Separator), "_")
	safeHost = strings.ReplaceAll(safeHost, ":", "_")
	fileName := fmt.Sprintf("%s-%06d-%s-%s.txt", ts, idx, strings.ToUpper(r.Method), safeHost)
	path := filepath.Join(dir, fileName)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	_, _ = w.WriteString("[RAW REQUEST ]\n")
	_, _ = w.Write(reqDump)
	if len(reqDump) > 0 && !bytes.HasSuffix(reqDump, []byte("\n")) {
		_, _ = w.WriteString("\n")
	}
	_, _ = w.WriteString(divider)
	_, _ = w.WriteString("[RAW RESPONSE]\n")
	_, _ = w.Write(respDump)
	if len(respDump) > 0 && !bytes.HasSuffix(respDump, []byte("\n")) {
		_, _ = w.WriteString("\n")
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return path, nil
}

// StartCaptureFile writes the request section and response headers, returning the file path and writer.
func StartCaptureFile(r *http.Request, reqDump []byte, respHeader []byte, config *Config) (string, *bufio.Writer, *os.File, error) {
	ts := time.Now().UTC().Format("20060102-150405.000000000")
	idx := config.ReqCounter.Add(1)
	safeHost := strings.ReplaceAll(r.Host, string(filepath.Separator), "_")
	safeHost = strings.ReplaceAll(safeHost, ":", "_")
	fileName := fmt.Sprintf("%s-%06d-%s-%s.txt", ts, idx, strings.ToUpper(r.Method), safeHost)
	path := filepath.Join(config.OutputDir, fileName)

	f, err := os.Create(path)
	if err != nil {
		return "", nil, nil, err
	}
	w := bufio.NewWriter(f)
	_, _ = w.WriteString("[RAW REQUEST ]\n")
	_, _ = w.Write(reqDump)
	if len(reqDump) > 0 && !bytes.HasSuffix(reqDump, []byte("\n")) {
		_, _ = w.WriteString("\n")
	}
	_, _ = w.WriteString(divider)
	_, _ = w.WriteString("[RAW RESPONSE]\n")
	_, _ = w.Write(respHeader)
	if len(respHeader) > 0 && !bytes.HasSuffix(respHeader, []byte("\n")) {
		_, _ = w.WriteString("\n")
	}
	return path, w, f, nil
}

func CloseCapture(w *bufio.Writer, f *os.File) error {
	if w != nil {
		if err := w.Flush(); err != nil {
			_ = f.Close()
			return err
		}
	}
	return f.Close()
}
