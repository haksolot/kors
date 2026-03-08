package postgres_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/adapter/postgres"
    "github.com/haksolot/kors/kors-api/internal/domain/identity"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/haksolot/kors/kors-api/internal/testhelper"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestPermissionRepository(t *testing.T) {
    pool := testhelper.SetupTestDB(t)
    ctx := context.Background()

    pRepo := &postgres.PermissionRepository{Pool: pool}
    idRepo := &postgres.IdentityRepository{Pool: pool}

    t.Run("Create and Check", func(t *testing.T) {
        identityID := uuid.New()
        // Must create identity first due to FK constraint
        err := idRepo.Create(ctx, &identity.Identity{
            ID: identityID, ExternalID: "test-user", Name: "Test User", Type: "user", CreatedAt: time.Now(),
        })
        require.NoError(t, err)

        p := &permission.Permission{
            ID:         uuid.New(),
            IdentityID: identityID,
            Action:     "read",
            CreatedAt:  time.Now().Truncate(time.Microsecond),
        }
        require.NoError(t, pRepo.Create(ctx, p))

        allowed, err := pRepo.Check(ctx, identityID, "read", nil, nil)
        require.NoError(t, err)
        assert.True(t, allowed)
    })

    t.Run("Check expired permission", func(t *testing.T) {
        identityID := uuid.New()
        err := idRepo.Create(ctx, &identity.Identity{
            ID: identityID, ExternalID: "test-user-expired", Name: "Test User Expired", Type: "user", CreatedAt: time.Now(),
        })
        require.NoError(t, err)

        past := time.Now().Add(-time.Hour)
        p := &permission.Permission{
            ID:         uuid.New(),
            IdentityID: identityID,
            Action:     "write",
            ExpiresAt:  &past,
            CreatedAt:  time.Now().Truncate(time.Microsecond),
        }
        require.NoError(t, pRepo.Create(ctx, p))

        allowed, err := pRepo.Check(ctx, identityID, "write", nil, nil)
        require.NoError(t, err)
        assert.False(t, allowed)
    })

    t.Run("Delete permission", func(t *testing.T) {
        identityID := uuid.New()
        err := idRepo.Create(ctx, &identity.Identity{
            ID: identityID, ExternalID: "test-user-delete", Name: "Test User Delete", Type: "user", CreatedAt: time.Now(),
        })
        require.NoError(t, err)

        p := &permission.Permission{
            ID:         uuid.New(),
            IdentityID: identityID,
            Action:     "transition",
            CreatedAt:  time.Now().Truncate(time.Microsecond),
        }
        require.NoError(t, pRepo.Create(ctx, p))

        require.NoError(t, pRepo.Delete(ctx, p.ID))

        allowed, err := pRepo.Check(ctx, identityID, "transition", nil, nil)
        require.NoError(t, err)
        assert.False(t, allowed)
    })
}
