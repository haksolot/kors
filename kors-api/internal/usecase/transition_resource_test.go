package usecase_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/resource"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
    "github.com/haksolot/kors/kors-api/internal/usecase"
    "github.com/haksolot/kors/kors-api/internal/usecase/mocks"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestTransitionResourceUseCase(t *testing.T) {
    ctx := context.Background()
    callerID := uuid.New()
    rt := sampleResourceType()
    resID := uuid.New()
    existingRes := &resource.Resource{
        ID: resID, TypeID: rt.ID, State: "idle", Metadata: make(map[string]interface{}),
        CreatedAt: time.Now(), UpdatedAt: time.Now(),
    }

    t.Run("success", func(t *testing.T) {
        uc := &usecase.TransitionResourceUseCase{
            ResourceRepo:     &mocks.ResourceRepo{Resources: map[uuid.UUID]*resource.Resource{resID: existingRes}},
            ResourceTypeRepo: &mocks.ResourceTypeRepo{Types: map[string]*resourcetype.ResourceType{rt.Name: rt}},
            EventRepo:        &mocks.EventRepo{},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: true},
            EventPublisher:   &mocks.EventPublisher{},
        }
        res, err := uc.Execute(ctx, usecase.TransitionResourceInput{
            ResourceID: resID, ToState: "in_use", IdentityID: callerID,
        })
        require.NoError(t, err)
        assert.Equal(t, "in_use", res.State)
    })

    t.Run("resource not found", func(t *testing.T) {
        uc := &usecase.TransitionResourceUseCase{
            ResourceRepo: &mocks.ResourceRepo{},
        }
        res, err := uc.Execute(ctx, usecase.TransitionResourceInput{
            ResourceID: resID, ToState: "in_use", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
    })

    t.Run("permission denied", func(t *testing.T) {
        uc := &usecase.TransitionResourceUseCase{
            ResourceRepo:   &mocks.ResourceRepo{Resources: map[uuid.UUID]*resource.Resource{resID: existingRes}},
            PermissionRepo: &mocks.PermissionRepo{AllowAll: false},
        }
        res, err := uc.Execute(ctx, usecase.TransitionResourceInput{
            ResourceID: resID, ToState: "in_use", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
    })

    t.Run("transition not allowed", func(t *testing.T) {
        uc := &usecase.TransitionResourceUseCase{
            ResourceRepo:     &mocks.ResourceRepo{Resources: map[uuid.UUID]*resource.Resource{resID: existingRes}},
            ResourceTypeRepo: &mocks.ResourceTypeRepo{Types: map[string]*resourcetype.ResourceType{rt.Name: rt}},
            PermissionRepo:   &mocks.PermissionRepo{AllowAll: true},
        }
        res, err := uc.Execute(ctx, usecase.TransitionResourceInput{
            ResourceID: resID, ToState: "invalid_state", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, res)
    })
}
