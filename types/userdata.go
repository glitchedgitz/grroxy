package types

import (
	"encoding/json"
	"fmt"
)

type RequestData struct {
	Url        string `db:"url" json:"url"`
	Path       string `db:"path" json:"path"`
	Query      string `db:"query" json:"query"`
	Fragment   string `db:"fragment" json:"fragment"`
	Ext        string `db:"ext" json:"ext"`
	Method     string `db:"method" json:"method"`
	HasCookies bool   `db:"has_cookies" json:"has_cookies"`
	HasParams  bool   `db:"has_params" json:"has_params"`
	Length     int    `db:"length" json:"length"`
	IsHTTPS    bool   `db:"is_https" json:"is_https"`
}

type ResponseData struct {
	Title      string `db:"title" json:"title"`
	Mime       string `db:"mime" json:"mime"`
	Status     int    `db:"status" json:"status"`
	Length     int    `db:"length" json:"length"`
	HasCookies bool   `db:"has_cookies" json:"has_cookies"`
	Date       string `db:"date" json:"date"`
	Time       string `db:"time" json:"time"`
}

type UserData struct {
	ID           string       `db:"id,omitempty" json:"id,omitempty"`
	Host         string       `db:"host,omitempty" json:"host,omitempty"`
	Index        int          `db:"index,omitempty" json:"index,omitempty"`
	Port         string       `db:"port,omitempty" json:"port,omitempty"`
	HasResp      bool         `db:"has_resp,omitempty" json:"has_resp,omitempty"`
	IsReqEdited  bool         `db:"is_req_edited,omitempty" json:"is_req_edited,omitempty"`
	IsRespEdited bool         `db:"is_resp_edited,omitempty" json:"is_resp_edited,omitempty"`
	Req          RequestData  `db:"req" json:"req"`
	Resp         ResponseData `db:"resp" json:"resp"`
	ReqEdited    RequestData  `db:"req_edited,omitempty" json:"req_edited,omitempty"`
	RespEdited   ResponseData `db:"resp_edited,omitempty" json:"resp_edited,omitempty"`
	Raw          string       `db:"raw,omitempty" json:"raw,omitempty"`
	Attached     string       `db:"attached,omitempty" json:"attached,omitempty"`

	// Action didn't get saved anywhere, it for intercept forward/drop. Although for below {RealtimeRecord} it's saved in `_intercept` collection.
	Action string `db:"action,omitempty" json:"action,omitempty"`
}

type RealtimeRecord struct {
	CollectionId   string      `db:"collectionId" json:"collectionId"`
	CollectionName string      `db:"collectionName" json:"collectionName"`
	Created        string      `db:"created" json:"created"`
	Index          int         `db:"index" json:"index"`
	Updated        string      `db:"updated" json:"updated"`
	ID             string      `db:"id" json:"id"`
	Host           string      `db:"host" json:"host"`
	Port           string      `db:"port" json:"port"`
	Req            interface{} `db:"req" json:"req"`
	Resp           interface{} `db:"resp" json:"resp"`
	HasResp        bool        `db:"has_resp" json:"has_resp"`
	IsReqEdited    bool        `db:"is_req_edited" json:"is_req_edited"`
	IsRespEdited   bool        `db:"is_resp_edited" json:"is_resp_edited"`
	ReqEdited      interface{} `db:"req_edited" json:"req_edited"`
	RespEdited     interface{} `db:"resp_edited" json:"resp_edited"`
	Raw            interface{} `db:"raw,omitempty" json:"raw,omitempty"`
	Attached       string      `db:"attached,omitempty" json:"attached,omitempty"`
	Action         string      `db:"action,omitempty" json:"action,omitempty"`
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
