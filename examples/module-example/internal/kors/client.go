package kors

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/machinebox/graphql"
)

type Client struct {
	gql   *graphql.Client
	token string
}

func NewClient(url string, token string) *Client {
	return &Client{
		gql:   graphql.NewClient(url),
		token: token,
	}
}

func (c *Client) CreateResource(ctx context.Context, typeName string, state string, meta map[string]interface{}) (uuid.UUID, error) {
	req := graphql.NewRequest(`
		mutation($input: CreateResourceInput!) {
			createResource(input: $input) {
				success
				resource { id }
				error { message }
			}
		}
	`)
	req.Var("input", map[string]interface{}{
		"typeName":     typeName,
		"initialState": state,
		"metadata":     meta,
	})
	req.Header.Set("Authorization", "Bearer "+c.token)

	var resp struct {
		CreateResource struct {
			Success  bool `json:"success"`
			Resource struct {
				ID string `json:"id"`
			} `json:"resource"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"createResource"`
	}

	if err := c.gql.Run(ctx, req, &resp); err != nil {
		return uuid.Nil, err
	}

	if !resp.CreateResource.Success {
		return uuid.Nil, fmt.Errorf("KORS error: %s", resp.CreateResource.Error.Message)
	}

	return uuid.Parse(resp.CreateResource.Resource.ID)
}
