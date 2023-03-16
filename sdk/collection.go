package sdk

import (
	"encoding/json"
	"fmt"

	"github.com/glitchedgitz/grroxy-db/types"
)

type Collection[T any] struct {
	*Client
	Name string
}

func CollectionSet[T any](client *Client, collection string) Collection[T] {
	return Collection[T]{client, collection}
}

func (c Collection[T]) Update(id string, body T) error {
	return c.Client.Update(c.Name, id, body)
}

func (c Collection[T]) Create(body T) (types.ResponseCreate, error) {
	return c.Client.Create(c.Name, body)
}

func (c Collection[T]) Delete(id string) error {
	return c.Client.Delete(c.Name, id)
}

func (c Collection[T]) SitemapNew(data types.SitemapGet) error {
	return c.Client.SitemapNew(data)
}

func (c Collection[T]) List(params types.ParamsList) (types.ResponseList[T], error) {
	var response types.ResponseList[T]
	params.HackResponseRef = &response

	_, err := c.Client.List(c.Name, params)
	return response, err
}

func (c Collection[T]) One(id string) (T, error) {
	var response T

	if err := c.Authorize(); err != nil {
		return response, err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam("collection", c.Name).
		SetPathParam("id", id)

	resp, err := request.Get(c.url + "/api/collections/{collection}/records/{id}")
	if err != nil {
		return response, fmt.Errorf("[one] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return response, fmt.Errorf("[one] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		return response, fmt.Errorf("[one] can't unmarshal response, err %w", err)
	}
	return response, nil
}

// Fetch Sitemap
