package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type RequestData struct {
	Url        string            `db:"url" json:"url"`
	Path       string            `db:"path" json:"path"`
	Query      string            `db:"query" json:"query"`
	Headers    map[string]string `db:"headers" json:"headers"`
	Fragment   string            `db:"fragment" json:"fragment"`
	Ext        string            `db:"ext" json:"ext"`
	Method     string            `db:"method" json:"method"`
	HasCookies bool              `db:"has_cookies" json:"has_cookies"`
	HasParams  bool              `db:"has_params" json:"has_params"`
	Length     int64             `db:"length" json:"length"`
}

type ResponseData struct {
	Title      string            `db:"title" json:"title"`
	Mime       string            `db:"mime" json:"mime"`
	Status     int               `db:"status" json:"status"`
	Headers    map[string]string `db:"headers" json:"headers"`
	Length     int64             `db:"length" json:"length"`
	HasCookies bool              `db:"has_cookies" json:"has_cookies"`
	Date       string            `db:"date" json:"date"`
	Time       string            `db:"time" json:"time"`
}

type UserData struct {
	ID             string         `db:"id,omitempty" json:"id,omitempty"`
	Index          float64        `db:"index,omitempty" json:"index,omitempty"`
	IndexMinor     float64        `db:"index_minor,omitempty" json:"index_minor,omitempty"`
	Host           string         `db:"host,omitempty" json:"host,omitempty"`
	Port           string         `db:"port,omitempty" json:"port,omitempty"`
	HasResp        bool           `db:"has_resp,omitempty" json:"has_resp,omitempty"`
	HasParams      bool           `db:"has_params,omitempty" json:"has_params,omitempty"`
	IsReqEdited    bool           `db:"is_req_edited,omitempty" json:"is_req_edited,omitempty"`
	IsRespEdited   bool           `db:"is_resp_edited,omitempty" json:"is_resp_edited,omitempty"`
	IsHTTPS        bool           `db:"is_https" json:"is_https"`
	Http           string         `db:"http" json:"http"`
	ProxyId        string         `db:"proxy_id,omitempty" json:"proxy_id,omitempty"`
	Req            string         `db:"req" json:"req"`
	Resp           string         `db:"resp" json:"resp"`
	ReqEdited      string         `db:"req_edited,omitempty" json:"req_edited,omitempty"`
	RespEdited     string         `db:"resp_edited,omitempty" json:"resp_edited,omitempty"`
	ReqJson        RequestData    `db:"req_json" json:"req_json"`
	RespJson       ResponseData   `db:"resp_json" json:"resp_json"`
	ReqEditedJson  RequestData    `db:"req_edited_json,omitempty" json:"req_edited_json,omitempty"`
	RespEditedJson ResponseData   `db:"resp_edited_json,omitempty" json:"resp_edited_json,omitempty"`
	GeneratedBy    string         `db:"generated_by,omitempty" json:"generated_by,omitempty"`
	Extra          map[string]any `db:"extra,omitempty" json:"extra,omitempty"`
	Attached       string         `db:"attached,omitempty" json:"attached,omitempty"`

	// Action didn't get saved anywhere, it for intercept forward/drop. Although for below {RealtimeRecord} it's saved in `_intercept` collection.
	Action string `db:"action,omitempty" json:"action,omitempty"`
}

func (userdata *UserData) RequestUpdateKey(req *http.Request, key string, value any) {
	log.Println("[RequestUpdateKey] key: '", key, "' value: '", value, "'")
	if key == "req.method" {
		req.Method = value.(string)
		userdata.ReqJson.Method = value.(string)
	} else if key == "req.url" {
		parsedURL, err := url.Parse(value.(string))
		if err != nil {
			return
		}
		req.URL = parsedURL
		userdata.ReqJson.Url = parsedURL.RequestURI()

	} else if key == "req.path" {
		req.URL.Path = value.(string)
		userdata.ReqJson.Path = value.(string)

	} else if strings.HasPrefix(key, "req.query") {
		params := req.URL.Query()
		params.Set(key, value.(string))
		req.URL.RawQuery = params.Encode()

	} else if strings.HasPrefix(key, "req.headers") {
		log.Println("[RequestUpdateKey] req.headers: ", key, value)

		header := strings.TrimPrefix(key, "req.headers")[1:]
		req.Header.Set(header, value.(string))
		userdata.ReqJson.Headers[header] = value.(string)

	} else if key == "req.body" {
		newBody := value.(string)
		req.Body = io.NopCloser(bytes.NewBufferString(newBody))
		newLength := int64(len(newBody))
		req.ContentLength = newLength
		userdata.ReqJson.Length = newLength

	}
}

func (userdata *UserData) RequestDeleteKey(req *http.Request, key string) {
	if key == "req.method" {
		req.Method = "GET"

	} else if key == "req.url" {
		parsedURL, _ := url.Parse("")
		req.URL = parsedURL

	} else if key == "req.path" {
		req.URL.Path = ""

	} else if strings.HasPrefix(key, "req.query") {
		params := req.URL.Query()
		params.Del(key)
		req.URL.RawQuery = params.Encode()

	} else if strings.HasPrefix(key, "req.headers") {
		header := strings.TrimPrefix(key, "req.headers")[1:]
		req.Header.Del(header)
		delete(userdata.ReqJson.Headers, header)

	} else if key == "req.body" {
		newBody := ""
		req.Body = io.NopCloser(bytes.NewBufferString(newBody))
		newLength := int64(len(newBody))
		req.ContentLength = newLength
		userdata.ReqJson.Length = newLength
	}
}

func (userdata *UserData) ResponseUpdateKey(resp *http.Response, key string, value any) {
	if key == "resp.mime" {
		resp.Header.Set("Content-Type", value.(string))
		userdata.RespJson.Headers["Content-Type"] = value.(string)

	} else if key == "resp.status" {
		resp.StatusCode = value.(int)
		userdata.RespJson.Status = value.(int)

	} else if strings.HasPrefix(key, "resp.headers") {

		header := strings.TrimPrefix(key, "resp.headers")[1:]
		resp.Header.Set(header, value.(string))
		userdata.RespJson.Headers[header] = value.(string)

	} else if key == "resp.body" {
		newBody := value.(string)
		resp.Body = io.NopCloser(bytes.NewBufferString(newBody))
		newLength := int64(len(newBody))
		resp.ContentLength = newLength
		userdata.RespJson.Length = newLength

	}
}

func (userdata *UserData) ResponseDeleteKey(resp *http.Response, key string) {
	if key == "resp.mime" {
		resp.Header.Del("Content-Type")
		delete(userdata.RespJson.Headers, "Content-Type")

	} else if strings.HasPrefix(key, "resp.headers") {

		header := strings.TrimPrefix(key, "resp.headers")[1:]
		resp.Header.Del(header)
		delete(userdata.RespJson.Headers, header)

	} else if key == "resp.body" {
		newBody := ""
		resp.Body = io.NopCloser(bytes.NewBufferString(newBody))
		newLength := int64(len(newBody))
		resp.ContentLength = newLength
		userdata.RespJson.Length = newLength

	}
}

type RealtimeRecord struct {
	UserData

	CollectionId   string `db:"collectionId" json:"collectionId"`
	CollectionName string `db:"collectionName" json:"collectionName"`
	Created        string `db:"created" json:"created"`
	Index          int    `db:"index" json:"index"`
	Updated        string `db:"updated" json:"updated"`
	Action         string `db:"action,omitempty" json:"action,omitempty"`
	Raw            any    `db:"raw,omitempty" json:"raw,omitempty"`
}

type OutputData struct {
	Userdata UserData
	Host     string
	Port     string
	Folder   string
}

func (d *UserData) Scan(value interface{}) error {
	if value == nil {
		*d = UserData{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, d)
	case string:
		return json.Unmarshal([]byte(v), d)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

func (d *RealtimeRecord) Scan(value interface{}) error {
	if value == nil {
		*d = RealtimeRecord{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, d)
	case string:
		return json.Unmarshal([]byte(v), d)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

func (d *RequestData) Scan(value interface{}) error {
	if value == nil {
		*d = RequestData{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, d)
	case string:
		return json.Unmarshal([]byte(v), d)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

func (d *ResponseData) Scan(value interface{}) error {
	if value == nil {
		*d = ResponseData{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, d)
	case string:
		return json.Unmarshal([]byte(v), d)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

// {
//     "collectionId": "0rlm6bo4w4ldxzw",
//     "collectionName": "intercept",
//     "created": "2023-03-29 12:24:16.192Z",
//     "req_edited": {},
//     "resp_edited": {},
//     "has_resp": true,
//     "host": "test2",
//     "id": "kVGuQP8HqUJITn1",
//     "ip": "test3",
//     "is_req_edited": true,
//     "is_resp_edited": true,
//     "labels": {},
//     "req": {},
//     "resp": {},
//     "port": "test3",
//     "updated": "2023-03-29 12:36:40.444Z",
//     "url_data": {}
// }
