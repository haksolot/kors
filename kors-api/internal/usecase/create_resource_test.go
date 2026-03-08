package usecase_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
    "github.com/haksolot/kors/kors-api/internal/usecase"
    "github.com/haksolot/kors/kors-api/internal/usecase/mocks"
    "github.com/stretchr/testify/assert"
)

// pgxpool requires a real connection to begin a transaction, so mocking this fully
// pure unit test with the new pgx requirement is tricky without interfaces.
// However, I'll write the test to compile. To make it run we might need a db test.
// We'll skip it for pure domain logic or adapt if we can.
// Actually, since CreateResourceUseCase uses *pgxpool.Pool directly and casts to *postgres.ResourceRepository,
// it is difficult to test purely with mocks.
// So let's provide a basic test structure as requested by instructions but note the postgres dependency.
// Wait, the prompt provided the exact code for usecase_test. Let me use it and fix compilation.
// Actually, since we modified CreateResourceUseCase to use pgxpool.Pool directly, testing it with mocks
// is impossible unless we pass nil and recover, or ignore.
// The prompt asked to create create_resource_test.go exactly like this:

func sampleResourceType() *resourcetype.ResourceType {
    return &resourcetype.ResourceType{
        ID:   uuid.New(),
        Name: "cnc_machine",
        JSONSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "serial": map[string]interface{}{"type": "string"},
            },
        },
        Transitions: map[string]interface{}{
            "idle":   []interface{}{"in_use"},
            "in_use": []interface{}{"idle"},
        },
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
}

func TestCreateResourceUseCase(t *testing.T) {
    ctx := context.Background()
    callerID := uuid.New()

    t.Run("resource type not found", func(t *testing.T) {
        uc := &usecase.CreateResourceUseCase{
            ResourceTypeRepo: &mocks.ResourceTypeRepo{},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: true},
        }
        res, err := uc.Execute(ctx, usecase.CreateResourceInput{
            TypeName: "nonexistent", InitialState: "idle", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
        assert.Contains(t, err.Error(), "not found")
    })

    t.Run("permission denied", func(t *testing.T) {
        rt := sampleResourceType()
        uc := &usecase.CreateResourceUseCase{
            ResourceTypeRepo: &mocks.ResourceTypeRepo{Types: map[string]*resourcetype.ResourceType{rt.Name: rt}},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: false},
        }
        res, err := uc.Execute(ctx, usecase.CreateResourceInput{
            TypeName: rt.Name, InitialState: "idle", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
        assert.Contains(t, err.Error(), "permission")
    })

    t.Run("metadata schema validation fails", func(t *testing.T) {
        rt := sampleResourceType()
        uc := &usecase.CreateResourceUseCase{
            ResourceTypeRepo: &mocks.ResourceTypeRepo{Types: map[string]*resourcetype.ResourceType{rt.Name: rt}},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: true},
        }
        res, err := uc.Execute(ctx, usecase.CreateResourceInput{
            TypeName:     rt.Name,
            InitialState: "idle",
            Metadata:     map[string]interface{}{"serial": 999},
            IdentityID:   callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
        assert.Contains(t, err.Error(), "metadata")
    })
}
