package usecase_test

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/usecase"
    "github.com/haksolot/kors/kors-api/internal/usecase/mocks"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRegisterResourceTypeUseCase(t *testing.T) {
    ctx := context.Background()
    callerID := uuid.New()

    t.Run("success", func(t *testing.T) {
        uc := &usecase.RegisterResourceTypeUseCase{
            Repo:           &mocks.ResourceTypeRepo{},
            PermissionRepo: &mocks.PermissionRepo{AllowAll: true},
        }
        rt, err := uc.Execute(ctx, usecase.RegisterResourceTypeInput{
            Name: "new_type", IdentityID: callerID, JSONSchema: map[string]interface{}{}, Transitions: map[string]interface{}{},
        })
        require.NoError(t, err)
        assert.NotNil(t, rt)
        assert.Equal(t, "new_type", rt.Name)
    })

    t.Run("permission denied", func(t *testing.T) {
        uc := &usecase.RegisterResourceTypeUseCase{
            Repo:           &mocks.ResourceTypeRepo{},
            PermissionRepo: &mocks.PermissionRepo{AllowAll: false},
        }
        rt, err := uc.Execute(ctx, usecase.RegisterResourceTypeInput{
            Name: "new_type", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, rt)
    })

    t.Run("empty name", func(t *testing.T) {
        uc := &usecase.RegisterResourceTypeUseCase{
            Repo:           &mocks.ResourceTypeRepo{},
            PermissionRepo: &mocks.PermissionRepo{AllowAll: true},
        }
        rt, err := uc.Execute(ctx, usecase.RegisterResourceTypeInput{
            Name: "", IdentityID: callerID,
        })
        assert.Error(t, err)
        assert.Nil(t, rt)
    })
}
