package pagination

import (
	"encoding/base64"
	"fmt"
)

// PageInfo contains metadata about a paginated result.
type PageInfo struct {
	HasNextPage     bool
	HasPreviousPage bool
	StartCursor     *string
	EndCursor       *string
}

// EncodeCursor encodes a string ID into a base64 cursor.
func EncodeCursor(id string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("cursor:%s", id)))
}

// DecodeCursor decodes a base64 cursor back to its ID.
func DecodeCursor(cursor string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	var id string
	_, err = fmt.Sscanf(string(b), "cursor:%s", &id)
	return id, err
}
