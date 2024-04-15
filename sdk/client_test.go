// file: client.go

package sdk

import (
	"testing"
	"time"

	"github.com/glitchedgitz/grroxy-db/base"
	"github.com/glitchedgitz/grroxy-db/types"
	// "github.com/r--w/pocketbase/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultURL = "http://127.0.0.1:8090"
)

// REMEMBER to start the Pocketbase before running this example with `make serve` command

func TestAuthorizeAnonymous(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Empty credentials",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(defaultURL)
			err := c.Authorize()
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestListAccess(t *testing.T) {
	type auth struct {
		email    string
		password string
	}
	tests := []struct {
		name       string
		admin      auth
		user       auth
		collection string
		wantResult bool
		wantErr    bool
	}{
		{
			name:       "With admin credentials - posts_admin",
			admin:      auth{email: migrations.AdminEmailPassword, password: migrations.AdminEmailPassword},
			collection: migrations.PostsAdmin,
			wantResult: true,
			wantErr:    false,
		},
		{
			name:       "Without credentials - posts_admin",
			collection: migrations.PostsAdmin,
			wantErr:    true,
		},
		{
			name:       "Without credentials - posts_public",
			collection: migrations.PostsPublic,
			wantResult: true,
			wantErr:    false,
		},
		{
			// For access rule @request.auth.id != ""
			// no error is returned, but empty result
			name:       "Without credentials - posts_user",
			collection: migrations.PostsUser,
			wantResult: false,
			wantErr:    false,
		},
		{
			name:       "With user credentials - posts_user",
			user:       auth{email: migrations.UserEmailPassword, password: migrations.UserEmailPassword},
			collection: migrations.PostsUser,
			wantResult: true,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(defaultURL)
			if tt.admin.email != "" {
				c = NewClient(defaultURL, WithAdminEmailPassword(tt.admin.email, tt.admin.password))
			} else if tt.user.email != "" {
				c = NewClient(defaultURL, WithUserEmailPassword(tt.user.email, tt.user.password))
			}
			r, err := c.List(tt.collection, types.ParamsList{})
			assert.Equal(t, tt.wantErr, err != nil, err)
			assert.Equal(t, tt.wantResult, r.TotalItems > 0)
		})
	}
}

func TestAuthorizeEmailPassword(t *testing.T) {
	type args struct {
		email    string
		password string
	}
	tests := []struct {
		name    string
		admin   args
		user    args
		wantErr bool
	}{
		{
			name:    "Valid credentials admin",
			admin:   args{email: migrations.AdminEmailPassword, password: migrations.AdminEmailPassword},
			wantErr: false,
		},
		{
			name:    "Invalid credentials admin",
			admin:   args{email: "invalid_" + migrations.AdminEmailPassword, password: "no_admin@admin.com"},
			wantErr: true,
		},
		{
			name:    "Valid credentials user",
			user:    args{email: migrations.UserEmailPassword, password: migrations.UserEmailPassword},
			wantErr: false,
		},
		{
			name:    "Invalid credentials user",
			user:    args{email: "invalid_" + migrations.UserEmailPassword, password: migrations.UserEmailPassword},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(defaultURL)
			if tt.admin.email != "" {
				c = NewClient(defaultURL, WithAdminEmailPassword(tt.admin.email, tt.admin.password))
			} else if tt.user.email != "" {
				c = NewClient(defaultURL, WithUserEmailPassword(tt.user.email, tt.user.password))
			}
			err := c.Authorize()
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestClient_List(t *testing.T) {
	defaultClient := NewClient(defaultURL)

	tests := []struct {
		name       string
		client     *Client
		collection string
		params     types.ParamsList
		wantResult bool
		wantErr    bool
	}{
		{
			name:       "List with no params",
			client:     defaultClient,
			collection: migrations.PostsPublic,
			wantErr:    false,
			wantResult: true,
		},
		{
			name:       "List no results - query",
			client:     defaultClient,
			collection: migrations.PostsPublic,
			params: types.ParamsList{
				Filters: "field='some_random_value'",
			},
			wantErr:    false,
			wantResult: false,
		},
		{
			name:       "List no results - invalid query",
			client:     defaultClient,
			collection: migrations.PostsPublic,
			params: types.ParamsList{
				Filters: "field~~~some_random_value'",
			},
			wantErr:    true,
			wantResult: false,
		},
		{
			name:       "List invalid collection",
			client:     defaultClient,
			collection: "invalid_collection",
			wantErr:    true,
			wantResult: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.client.List(tt.collection, tt.params)
			assert.Equal(t, tt.wantErr, err != nil, err)
			assert.Equal(t, tt.wantResult, got.TotalItems > 0)
		})
	}
}

func TestClient_Delete(t *testing.T) {
	client := NewClient(defaultURL)
	field := "value_" + time.Now().Format(time.StampMilli)

	// delete non-existing item
	err := client.Delete(migrations.PostsPublic, "non_existing_id")
	assert.Error(t, err)

	// create temporary item
	resultCreated, err := client.Create(migrations.PostsPublic, map[string]any{
		"field": field,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, resultCreated.ID)

	// confirm item exists
	resultList, err := client.List(migrations.PostsPublic, types.ParamsList{Filters: "id='" + resultCreated.ID + "'"})
	assert.NoError(t, err)
	assert.Len(t, resultList.Items, 1)

	// delete temporary item
	err = client.Delete(migrations.PostsPublic, resultCreated.ID)
	assert.NoError(t, err)

	// confirm item does not exist
	resultList, err = client.List(migrations.PostsPublic, types.ParamsList{Filters: "id='" + resultCreated.ID + "'"})
	assert.NoError(t, err)
	assert.Len(t, resultList.Items, 0)
}

func TestClient_Update(t *testing.T) {
	client := NewClient(defaultURL)
	field := "value_" + time.Now().Format(time.StampMilli)

	// update non-existing item
	err := client.Update(migrations.PostsPublic, "non_existing_id", map[string]any{
		"field": field,
	})
	assert.Error(t, err)

	// create temporary item
	resultCreated, err := client.Create(migrations.PostsPublic, map[string]any{
		"field": field,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, resultCreated.ID)

	// confirm item exists
	resultList, err := client.List(migrations.PostsPublic, types.ParamsList{Filters: "id='" + resultCreated.ID + "'"})
	assert.NoError(t, err)
	require.Len(t, resultList.Items, 1)
	assert.Equal(t, field, resultList.Items[0]["field"])

	// update temporary item
	err = client.Update(migrations.PostsPublic, resultCreated.ID, map[string]any{
		"field": field + "_updated",
	})
	assert.NoError(t, err)

	// confirm changes
	resultList, err = client.List(migrations.PostsPublic, types.ParamsList{Filters: "id='" + resultCreated.ID + "'"})
	assert.NoError(t, err)
	require.Len(t, resultList.Items, 1)
	assert.Equal(t, field+"_updated", resultList.Items[0]["field"])
}

func TestClient_Create(t *testing.T) {
	defaultClient := NewClient(defaultURL)
	defaultBody := map[string]interface{}{
		"field": "value_" + time.Now().Format(time.StampMilli),
	}

	tests := []struct {
		name       string
		client     *Client
		collection string
		body       any
		wantErr    bool
		wantID     bool
	}{
		{
			name:       "Create with no body",
			client:     defaultClient,
			collection: migrations.PostsPublic,
			wantErr:    true,
		},
		{
			name:       "Create with body",
			client:     defaultClient,
			collection: migrations.PostsPublic,
			body:       defaultBody,
			wantErr:    false,
			wantID:     true,
		},
		{
			name:       "Create invalid collections",
			client:     defaultClient,
			collection: "invalid_collection",
			body:       defaultBody,
			wantErr:    true,
		},
		{
			name:       "Create no auth",
			client:     defaultClient,
			collection: migrations.PostsUser,
			body:       defaultBody,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := tt.client.Create(tt.collection, tt.body)
			assert.Equal(t, tt.wantErr, err != nil, err)
			assert.Equal(t, tt.wantID, r.ID != "")
		})
	}
}

// Create test function for SitemapNew by suppling values of types.SitemapGet

func TestClient_SitemapNew(t *testing.T) {
	defaultClient := NewClient(defaultURL)
	tests := []struct {
		name    string
		client  *Client
		body    types.SitemapGet
		wantErr bool
		wantID  bool
	}{
		// {
		// 	name:       "Create with no body",
		// 	client:     defaultClient,
		// 	wantErr:    true,
		// },
		{
			name:   "Create with body",
			client: defaultClient,
			body: types.SitemapGet{
				Host:     "https://2example2.com",
				Path:     "/folder/subfolder/test",
				Type:     "file",
				Query:    "?test=1&test2=2",
				Fragment: "#frag",
				Data:     base.RandomString(15),
			},
			wantErr: false,
			wantID:  true,
		},
		// {
		// 	name:       "Create invalid collections",
		// 	client:     defaultClient,
		// 	body: types.SitemapGet{
		// 		Host: "",
		// 	},
		// 	wantErr: true,
		// },
		// {
		// 	name:       "Create no auth",
		// 	client:     defaultClient,
		// 	body: types.SitemapGet{
		// 		Host: "",
		// 	},
		// 	wantErr: true,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.SitemapNew(tt.body)
			assert.Equal(t, tt.wantErr, err != nil, err)
			// assert.Equal(t, tt.wantID, r.ID != "")
		})
	}
}
