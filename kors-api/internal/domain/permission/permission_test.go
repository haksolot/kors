package permission_test

import (
    "testing"
    "time"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/stretchr/testify/assert"
)

func TestPermissionIsExpired(t *testing.T) {
    t.Run("no expiry = never expired", func(t *testing.T) {
        p := &permission.Permission{ID: uuid.New(), ExpiresAt: nil}
        assert.False(t, p.IsExpired())
    })

    t.Run("future expiry = not expired", func(t *testing.T) {
        future := time.Now().Add(time.Hour)
        p := &permission.Permission{ID: uuid.New(), ExpiresAt: &future}
        assert.False(t, p.IsExpired())
    })

    t.Run("past expiry = expired", func(t *testing.T) {
        past := time.Now().Add(-time.Hour)
        p := &permission.Permission{ID: uuid.New(), ExpiresAt: &past}
        assert.True(t, p.IsExpired())
    })
}
