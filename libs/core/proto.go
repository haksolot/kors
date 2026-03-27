package core

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// Marshal encodes a Protobuf message to bytes.
// Returns a wrapped error with context on failure.
func Marshal(msg proto.Message) ([]byte, error) {
	b, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("Marshal: %T: %w", msg, err)
	}
	return b, nil
}

// Unmarshal decodes bytes into a Protobuf message.
// The caller must pass a non-nil pointer to the target message type.
func Unmarshal(b []byte, msg proto.Message) error {
	if err := proto.Unmarshal(b, msg); err != nil {
		return fmt.Errorf("Unmarshal: %T: %w", msg, err)
	}
	return nil
}
