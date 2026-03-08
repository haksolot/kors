package postgres_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/adapter/postgres"
    "github.com/haksolot/kors/kors-api/internal/domain/resource"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
    "github.com/haksolot/kors/kors-api/internal/testhelper"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestResourceRepository(t *testing.T) {
    pool := testhelper.SetupTestDB(t)
    ctx := context.Background()

    rtRepo := &postgres.ResourceTypeRepository{Pool: pool}
    rRepo := &postgres.ResourceRepository{Pool: pool}

    // Setup: creer un type de resource
    rt := &resourcetype.ResourceType{
        ID: uuid.New(), Name: "test_type",
        JSONSchema: map[string]interface{}{"type": "object"},
        Transitions: map[string]interface{}{"idle": []interface{}{"in_use"}},
        CreatedAt: time.Now().Truncate(time.Microsecond),
        UpdatedAt: time.Now().Truncate(time.Microsecond),
    }
    require.NoError(t, rtRepo.Create(ctx, rt))

    t.Run("Create and GetByID", func(t *testing.T) {
        res := &resource.Resource{
            ID: uuid.New(), TypeID: rt.ID, State: "idle",
            Metadata: map[string]interface{}{"key": "value"},
            CreatedAt: time.Now().Truncate(time.Microsecond),
            UpdatedAt: time.Now().Truncate(time.Microsecond),
        }
        require.NoError(t, rRepo.Create(ctx, res))

        found, err := rRepo.GetByID(ctx, res.ID)
        require.NoError(t, err)
        require.NotNil(t, found)
        assert.Equal(t, res.ID, found.ID)
        assert.Equal(t, "idle", found.State)
    })

    t.Run("GetByID not found returns nil", func(t *testing.T) {
        found, err := rRepo.GetByID(ctx, uuid.New())
        assert.NoError(t, err)
        assert.Nil(t, found)
    })

    t.Run("Update state", func(t *testing.T) {
        res := &resource.Resource{
            ID: uuid.New(), TypeID: rt.ID, State: "idle",
            Metadata: map[string]interface{}{},
            CreatedAt: time.Now().Truncate(time.Microsecond),
            UpdatedAt: time.Now().Truncate(time.Microsecond),
        }
        require.NoError(t, rRepo.Create(ctx, res))
        res.State = "in_use"
        res.UpdatedAt = time.Now().Truncate(time.Microsecond)
        require.NoError(t, rRepo.Update(ctx, res))

        found, _ := rRepo.GetByID(ctx, res.ID)
        assert.Equal(t, "in_use", found.State)
    })

    t.Run("List with pagination", func(t *testing.T) {
        // Creer 5 resources
        for i := 0; i < 5; i++ {
            r := &resource.Resource{
                ID: uuid.New(), TypeID: rt.ID, State: "idle",
                Metadata: map[string]interface{}{},
                CreatedAt: time.Now().Truncate(time.Microsecond),
                UpdatedAt: time.Now().Truncate(time.Microsecond),
            }
            require.NoError(t, rRepo.Create(ctx, r))
        }
        results, hasNext, total, err := rRepo.List(ctx, 3, nil, nil)
        require.NoError(t, err)
        assert.Len(t, results, 3)
        assert.True(t, hasNext)
        assert.GreaterOrEqual(t, total, 5)
    })

    t.Run("List filter by typeName", func(t *testing.T) {
        typeName := "test_type"
        results, _, _, err := rRepo.List(ctx, 100, nil, &typeName)
        require.NoError(t, err)
        for _, r := range results {
            assert.Equal(t, rt.ID, r.TypeID)
        }
    })
}
