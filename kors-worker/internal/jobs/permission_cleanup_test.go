package jobs_test

import (
    "context"
    "errors"
    "testing"

    "github.com/haksolot/kors/kors-worker/internal/jobs"
    "github.com/stretchr/testify/assert"
)

type MockPermissionRepo struct {
    CleanupCount int64
    CleanupErr   error
}

func (m *MockPermissionRepo) CleanupExpired(ctx context.Context) (int64, error) {
    return m.CleanupCount, m.CleanupErr
}

func TestPermissionCleanupJob(t *testing.T) {
    ctx := context.Background()

    t.Run("success", func(t *testing.T) {
        repo := &MockPermissionRepo{CleanupCount: 5}
        job := jobs.NewPermissionCleanupJob(repo)
        
        err := job.Run(ctx)
        assert.NoError(t, err)
    })

    t.Run("error locked", func(t *testing.T) {
        // Supposons que ErrLocked est defini quelque part ou on simule l'erreur
        repo := &MockPermissionRepo{CleanupErr: errors.New("locked")}
        job := jobs.NewPermissionCleanupJob(repo)
        
        err := job.Run(ctx)
        // Le job continue meme s'il y a erreur (selon la description) ou retourne l'erreur
        // On verifie juste que ca correspond au comportement voulu.
        assert.Error(t, err)
    })
}
