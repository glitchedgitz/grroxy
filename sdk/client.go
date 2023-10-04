package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/glitchedgitz/grroxy-db/types"
	"github.com/go-resty/resty/v2"
)

var ErrInvalidResponse = errors.New("invalid response")

type (
	Client struct {
		client     *resty.Client
		url        string
		authorizer authorizer
	}
	ClientOption func(*Client)
)

func NewClient(url string, opts ...ClientOption) *Client {
	client := resty.New()
	client.
		SetRetryCount(3).
		SetRetryWaitTime(3 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second)

	client.Debug = false

	c := &Client{
		client:     client,
		url:        url,
		authorizer: authorizeNoOp{},
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithDebug() ClientOption {
	return func(c *Client) {
		c.client.SetDebug(true)
	}
}

func WithAdminEmailPassword(email, password string) ClientOption {
	return func(c *Client) {
		c.authorizer = newAuthorizeEmailPassword(c.client, c.url+"/api/admins/auth-with-password", email, password)
	}
}

func WithUserEmailPassword(email, password string) ClientOption {
	return func(c *Client) {
		c.authorizer = newAuthorizeEmailPassword(c.client, c.url+"/api/collections/users/auth-with-password", email, password)
	}
}

func (c *Client) Authorize() error {
	return c.authorizer.authorize()
}

func (c *Client) Update(collection string, id string, body any) error {
	if err := c.Authorize(); err != nil {
		return err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam("collection", collection).
		SetBody(body)

	resp, err := request.Patch(c.url + "/api/collections/{collection}/records/" + id)
	if err != nil {
		return fmt.Errorf("[update] can't send update request to pocketbase, err %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("[update] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	return nil
}

func (c *Client) Create(collection string, body any) (types.ResponseCreate, error) {
	var response types.ResponseCreate

	if err := c.Authorize(); err != nil {
		return response, err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam("collection", collection).
		SetBody(body).
		SetResult(&response)

	resp, err := request.Post(c.url + "/api/collections/{collection}/records")
	if err != nil {
		return response, fmt.Errorf("[create] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return response, fmt.Errorf("[create] pocketbase returned status: %d, msg: %s, body: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			fmt.Sprintf("%+v", body), // TODO remove that after debugging
			ErrInvalidResponse,
		)
	}

	return *resp.Result().(*types.ResponseCreate), nil
}

func (c *Client) Delete(collection string, id string) error {
	if err := c.Authorize(); err != nil {
		return err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam("collection", collection).
		SetPathParam("id", id)

	resp, err := request.Delete(c.url + "/api/collections/{collection}/records/{id}")
	if err != nil {
		return fmt.Errorf("[delete] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("[delete] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	return nil
}

func (c *Client) List(collection string, params types.ParamsList) (types.ResponseList[map[string]any], error) {
	var response types.ResponseList[map[string]any]

	if err := c.Authorize(); err != nil {
		return response, err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam("collection", collection)

	if params.Page > 0 {
		request.SetQueryParam("page", convertor.ToString(params.Page))
	}
	if params.Size > 0 {
		request.SetQueryParam("perPage", convertor.ToString(params.Size))
	}
	if params.Filters != "" {
		request.SetQueryParam("filter", params.Filters)
	}
	if params.Sort != "" {
		request.SetQueryParam("sort", params.Sort)
	}

	resp, err := request.Get(c.url + "/api/collections/{collection}/records")
	if err != nil {
		return response, fmt.Errorf("[list] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return response, fmt.Errorf("[list] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	var responseRef any = &response
	if params.HackResponseRef != nil {
		responseRef = params.HackResponseRef
	}
	if err := json.Unmarshal(resp.Body(), responseRef); err != nil {
		return response, fmt.Errorf("[list] can't unmarshal response, err %w", err)
	}
	return response, nil
}

// // List All Collections from endpoint /api/collections
// func (c *Client) ListCollections() (types.ResponseList[types.Collection], error) {
// 	var response types.ResponseList[types.Collection]

// 	if err := c.Authorize(); err != nil {
// 		return response, err
// 	}

// 	request := c.client.R().
// 		SetHeader("Content-Type", "application/json")

// 	resp, err := request.Get(c.url + "/api/collections")
// 	if err != nil {
// 		return response, fmt.Errorf("[list] can't send update request to pocketbase, err %w", err)
// 	}

// 	if resp.IsError() {
// 		return response, fmt.Errorf("[list] pocketbase returned status: %d, msg: %s, err %w",
// 			resp.StatusCode(),
// 			resp.String(),
// 			ErrInvalidResponse,
// 		)
// 	}

// 	if err := json.Unmarshal(resp.Body(), &response); err != nil {
// 		return response, fmt.Errorf("[list] can't unmarshal response, err %w", err)
// 	}
// 	return response, nil
// }

// Create collection using endpoitn /api/collections using below javascrip example
// const base = await pb.collections.create({
//     name: 'exampleBase',
//     type: 'base',
//     schema: [
//         {
//             name: 'title',
//             type: 'text',
//             required: true,
//             options: {
//                 min: 10,
//             },
//         },
//         {
//             name: 'status',
//             type: 'bool',
//         },
//     ],
// });

func (c *Client) CreateCollection(data types.Collection) error {
	if err := c.Authorize(); err != nil {
		return err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(data)

	resp, err := request.Post(c.url + "/api/collections")
	if err != nil {
		return fmt.Errorf("[collection] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("[collection] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	return nil
}

// Sitemap New using endpoint form APIEndpoints[SitemapNew]
func (c *Client) SitemapNew(data types.SitemapGet) error {

	if err := c.Authorize(); err != nil {
		return err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(data)

	resp, err := request.Post(c.url + "/api/sitemap/new")
	if err != nil {
		return fmt.Errorf("[sitemap] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("[sitemap] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	return nil
}
