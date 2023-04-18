package types

import "net/url"

type RequestData struct {
	AllURL        url.URL `db:"all_url" json:"all_url"`
	Url           string  `db:"url" json:"url"`
	Method        string  `db:"method" json:"method"`
	HasCookies    bool    `db:"has_cookies" json:"has_cookies"`
	HasParams     bool    `db:"has_params" json:"has_params"`
	ContentLength int     `db:"content_length" json:"content_length"`
	IsHTTPS       bool    `db:"is_https" json:"is_https"`
	Date          string  `db:"date" json:"date"`
	Time          string  `db:"time" json:"time"`
}

type ResponseData struct {
	Title         string `db:"title" json:"title"`
	Mimetype      string `db:"mimetype" json:"mimetype"`
	StatusCode    int    `db:"status_code" json:"status_code"`
	ContentLength int64  `db:"content_length" json:"content_length"`
	HasCookies    bool   `db:"has_cookies" json:"has_cookies"`
	Date          string `db:"date" json:"date"`
	Time          string `db:"time" json:"time"`
}

type UrlData struct {
	Scheme string              `db:"scheme" json:"scheme"`
	Params map[string][]string `db:"params" json:"params"`
	Path   string              `db:"path" json:"path"`
}

type UserData struct {
	ID               string       `db:"id,omitempty" json:"id,omitempty"`
	Host             string       `db:"host,omitempty" json:"host,omitempty"`
	IP               string       `db:"ip,omitempty" json:"ip,omitempty"`
	Port             string       `db:"port,omitempty" json:"port,omitempty"`
	UrlData          UrlData      `db:"url_data,omitempty" json:"url_data,omitempty"`
	OriginalRequest  RequestData  `db:"original_request,omitempty" json:"original_request,omitempty"`
	OriginalResponse ResponseData `db:"original_response,omitempty" json:"original_response,omitempty"`
	HasResponse      bool         `db:"has_response,omitempty" json:"has_response,omitempty"`
	IsRequestEdited  bool         `db:"is_request_edited,omitempty" json:"is_request_edited,omitempty"`
	IsResponseEdited bool         `db:"is_response_edited,omitempty" json:"is_response_edited,omitempty"`
	EditedRequest    RequestData  `db:"edited_request,omitempty" json:"edited_request,omitempty"`
	EditedResponse   ResponseData `db:"edited_response,omitempty" json:"edited_response,omitempty"`
	Labels           []string     `db:"labels,omitempty" json:"labels,omitempty"`
}

type UserData2 struct {
	ID               string      `db:"id" json:"id"`
	Host             string      `db:"host" json:"host"`
	IP               string      `db:"ip" json:"ip"`
	Port             string      `db:"port" json:"port"`
	UrlData          interface{} `db:"url_data" json:"url_data"`
	OriginalRequest  interface{} `db:"original_request" json:"original_request"`
	OriginalResponse interface{} `db:"original_response" json:"original_response"`
	HasResponse      bool        `db:"has_response" json:"has_response"`
	IsRequestEdited  bool        `db:"is_request_edited" json:"is_request_edited"`
	IsResponseEdited bool        `db:"is_response_edited" json:"is_response_edited"`
	EditedRequest    interface{} `db:"edited_request" json:"edited_request"`
	EditedResponse   interface{} `db:"edited_response" json:"edited_response"`
	// Labels           []string     `db:"labels" json:"labels"`
}

// {
//     "collectionId": "0rlm6bo4w4ldxzw",
//     "collectionName": "intercept",
//     "created": "2023-03-29 12:24:16.192Z",
//     "edited_request": {},
//     "edited_response": {},
//     "has_response": true,
//     "host": "test2",
//     "id": "kVGuQP8HqUJITn1",
//     "ip": "test3",
//     "is_request_edited": true,
//     "is_response_edited": true,
//     "labels": {},
//     "original_request": {},
//     "original_response": {},
//     "port": "test3",
//     "updated": "2023-03-29 12:36:40.444Z",
//     "url_data": {}
// }

type RealtimeRecord struct {
	CollectionId     string      `db:"collectionId" json:"collectionId"`
	CollectionName   string      `db:"collectionName" json:"collectionName"`
	Created          string      `db:"created" json:"created"`
	Updated          string      `db:"updated" json:"updated"`
	ID               string      `db:"id" json:"id"`
	Host             string      `db:"host" json:"host"`
	IP               string      `db:"ip" json:"ip"`
	Port             string      `db:"port" json:"port"`
	UrlData          interface{} `db:"url_data" json:"url_data"`
	OriginalRequest  interface{} `db:"original_request" json:"original_request"`
	OriginalResponse interface{} `db:"original_response" json:"original_response"`
	HasResponse      bool        `db:"has_response" json:"has_response"`
	IsRequestEdited  bool        `db:"is_request_edited" json:"is_request_edited"`
	IsResponseEdited bool        `db:"is_response_edited" json:"is_response_edited"`
	EditedRequest    interface{} `db:"edited_request" json:"edited_request"`
	EditedResponse   interface{} `db:"edited_response" json:"edited_response"`
	Labels           interface{} `db:"labels,omitempty" json:"labels,omitempty"`
	Action           string      `db:"action,omitempty" json:"action,omitempty"`
}

type OutputData struct {
	Userdata UserData
	Host     string
	Port     string
	Folder   string
}

// type RequestData struct {
// 	AllURL        url.URL `db:"" json:""`
// 	Url           string  `db:"" json:""`
// 	Method        string  `db:"" json:""`
// 	HasCookies    bool    `db:"" json:""`
// 	HasParams     bool    `db:"" json:""`
// 	ContentLength int     `db:"" json:""`
// 	IsHTTPS       bool    `db:"" json:""`
// 	Date          string  `db:"" json:""`
// 	Time          string  `db:"" json:""`
// }

// type ResponseData struct {
// 	Title         string `db:"" json:""`
// 	Mimetype      string `db:"" json:""`
// 	StatusCode    int    `db:"" json:""`
// 	ContentLength int64  `db:"" json:""`
// 	HasCookies    bool   `db:"" json:""`
// 	Date          string `db:"" json:""`
// 	Time          string `db:"" json:""`
// }

// type EventData struct {
// 	ID   string `db:"" json:""`
// 	Data string `db:"" json:""`
// }

// type UrlData struct {
// 	Scheme string              `db:"" json:""`
// 	Params map[string][]string `db:"" json:""`
// 	Path   string              `db:"" json:""`
// }

// type UserData struct {
// 	ID               string       `db:"id" json:"id"`
// 	Host             string       `db:"" json:""`
// 	IP               string       `db:"" json:""`
// 	Port             string       `db:"" json:""`
// 	Event            EventData    `db:"" json:""`
// 	UrlData          UrlData      `db:"" json:""`
// 	OriginalRequest  RequestData  `db:"" json:""`
// 	OriginalResponse ResponseData `db:"" json:""`
// 	HasResponse      bool         `db:"" json:""`
// 	IsRequestEdited  bool         `db:"" json:""`
// 	IsResponseEdited bool         `db:"" json:""`
// 	EditedRequest    RequestData  `db:"" json:""`
// 	EditedResponse   ResponseData `db:"" json:""`
// 	Labels           []string     `db:"" json:""`
// }

// type RequestData struct {
// 	AllURL        url.URL `db:"all_url" json:"all_url"`
// 	Url           string  `db:"url" json:"url"`
// 	Method        string  `db:"method" json:"method"`
// 	HasCookies    bool    `db:"has_cookies" json:"has_cookies"`
// 	HasParams     bool    `db:"has_params" json:"has_params"`
// 	ContentLength int     `db:"content_length" json:"content_length"`
// 	IsHTTPS       bool    `db:"is_https" json:"is_https"`
// 	Date          string  `db:"date" json:"date"`
// 	Time          string  `db:"time" json:"time"`
// }

// type ResponseData struct {
// 	Title         string `db:"title" json:"title"`
// 	Mimetype      string `db:"mime_type" json:"mime_type"`
// 	StatusCode    int    `db:"status_code" json:"status_code"`
// 	ContentLength int64  `db:"content_length" json:"content_length"`
// 	HasCookies    bool   `db:"has_cookies" json:"has_cookies"`
// 	Date          string `db:"date" json:"date"`
// 	Time          string `db:"time" json:"time"`
// }

// type EventData struct {
// 	ID   string `db:"id" json:"id"`
// 	Data string `db:"data" json:"data"`
// }

// type UrlData struct {
// 	Scheme string              `db:"scheme" json:"scheme"`
// 	Params map[string][]string `db:"params" json:"params"`
// 	Path   string              `db:"path" json:"path"`
// }

// type UserData struct {
// 	ID               string       `db:"id" json:"id"`
// 	Host             string       `db:"host" json:"host"`
// 	IP               string       `db:"ip" json:"ip"`
// 	Port             string       `db:"port" json:"port"`
// 	Event            EventData    `db:"event" json:"event"`
// 	UrlData          UrlData      `db:"url_data" json:"url_data"`
// 	OriginalRequest  RequestData  `db:"original_request" json:"original_request"`
// 	OriginalResponse ResponseData `db:"original_response" json:"original_response"`
// 	HasResponse      bool         `db:"has_response" json:"has_response"`
// 	IsRequestEdited  bool         `db:"is_request_edited" json:"is_request_edited"`
// 	IsResponseEdited bool         `db:"is_response_edited" json:"is_response_edited"`
// 	EditedRequest    RequestData  `db:"edited_request" json:"edited_request"`
// 	EditedResponse   ResponseData `db:"edited_response" json:"edited_response"`
// 	Labels           []string     `db:"labels" json:"labels"`
// }
