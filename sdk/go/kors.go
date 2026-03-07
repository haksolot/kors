package sdk

import (
	"net/http"

	"github.com/Khan/genqlient/graphql"
)

// authedTransport is an http.RoundTripper that adds a Bearer token to requests.
type authedTransport struct {
	key     string
	wrapped http.RoundTripper
}

func (t *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.key)
	return t.wrapped.RoundTrip(req)
}

// Client is the KORS SDK client.
type Client struct {
	gql graphql.Client
}

// NewClient creates a new KORS SDK client.
func NewClient(endpoint string, token string) *Client {
	httpClient := &http.Client{
		Transport: &authedTransport{
			key:     token,
			wrapped: http.DefaultTransport,
		},
	}
	return &Client{
		gql: graphql.NewClient(endpoint, httpClient),
	}
}

// Internal access to the raw genqlient client if needed.
func (c *Client) GQL() graphql.Client {
	return c.gql
}

// Helper methods can be added here to simplify calls further.
// For now, developers can use the generated functions directly using c.gql.
