package resourcetype_test

import (
    "testing"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
    "github.com/stretchr/testify/assert"
)

func TestCanTransitionTo(t *testing.T) {
    rt := &resourcetype.ResourceType{
        Transitions: map[string]interface{}{
            "idle":        []interface{}{"in_use", "maintenance"},
            "in_use":      []interface{}{"idle", "error"},
            "maintenance": []interface{}{"idle"},
            "error":       []interface{}{"idle", "maintenance"},
        },
    }

    tests := []struct {
        name      string
        from, to  string
        expected  bool
    }{
        {"allowed: idle -> in_use", "idle", "in_use", true},
        {"allowed: idle -> maintenance", "idle", "maintenance", true},
        {"allowed: in_use -> error", "in_use", "error", true},
        {"denied: idle -> error (not in graph)", "idle", "error", false},
        {"denied: idle -> idle (self-loop not declared)", "idle", "idle", false},
        {"denied: unknown from state", "archived", "idle", false},
        {"denied: unknown to state", "idle", "nonexistent", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := rt.CanTransitionTo(tt.from, tt.to)
            assert.Equal(t, tt.expected, got)
        })
    }
}

func TestValidateMetadata(t *testing.T) {
    rt := &resourcetype.ResourceType{
        JSONSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "serial_number": map[string]interface{}{"type": "string"},
                "diameter_mm":   map[string]interface{}{"type": "number", "minimum": 0},
            },
            "required": []interface{}{"serial_number"},
        },
    }

    t.Run("valid metadata", func(t *testing.T) {
        err := rt.ValidateMetadata(map[string]interface{}{
            "serial_number": "SN-001",
            "diameter_mm":   12.5,
        })
        assert.NoError(t, err)
    })

    t.Run("missing required field", func(t *testing.T) {
        err := rt.ValidateMetadata(map[string]interface{}{
            "diameter_mm": 12.5,
        })
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "serial_number")
    })

    t.Run("wrong type", func(t *testing.T) {
        err := rt.ValidateMetadata(map[string]interface{}{
            "serial_number": 123, // doit etre string
        })
        assert.Error(t, err)
    })

    t.Run("empty schema = no constraint", func(t *testing.T) {
        rtNoSchema := &resourcetype.ResourceType{JSONSchema: map[string]interface{}{}}
        err := rtNoSchema.ValidateMetadata(map[string]interface{}{"anything": true})
        assert.NoError(t, err)
    })
}
