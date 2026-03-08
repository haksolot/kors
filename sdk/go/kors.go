package sdk

import (
	"context"
	"encoding/base64"
	"fmt"
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

// SaveFile uploads a file to KORS MinIO storage.
func (c *Client) SaveFile(ctx context.Context, fileName string, content []byte) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(content)
	
	// We use a raw request here to avoid dependency on regenerated code in this turn
	var resp struct {
		UploadFile struct {
			Success bool   `json:"success"`
			Url     string `json:"url"`
			Error   *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"uploadFile"`
	}

	query := `
		mutation UploadFile($input: UploadFileInput!) {
			uploadFile(input: $input) {
				success
				url
				error { message }
			}
		}
	`
	
	vars := map[string]interface{}{
		"input": map[string]interface{}{
			"fileName":    fileName,
			"fileContent": encoded,
			"contentType": "application/octet-stream",
		},
	}

	err := c.gql.MakeRequest(ctx, &graphql.Request{
		Query:     query,
		Variables: vars,
	}, &graphql.Response{Data: &resp})

	if err != nil {
		return "", err
	}

	if !resp.UploadFile.Success {
		if resp.UploadFile.Error != nil {
			return "", fmt.Errorf("KORS upload error: %s", resp.UploadFile.Error.Message)
		}
		return "", fmt.Errorf("KORS upload failed without specific error message")
	}

	return resp.UploadFile.Url, nil
}
