package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/haksolot/kors/kors-api/internal/adapter/postgres"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
	"github.com/haksolot/kors/kors-api/internal/testhelper"
	"github.com/haksolot/kors/kors-api/internal/usecase"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKorsCoreLifecycle_Integration(t *testing.T) {
	pool := testhelper.SetupTestDB(t)
	ctx := context.Background()

	// Initialisation des repositories réels
	rtRepo := &postgres.ResourceTypeRepository{Pool: pool}
	rRepo := &postgres.ResourceRepository{Pool: pool}
	eRepo := &postgres.EventRepository{Pool: pool}
	pRepo := &postgres.PermissionRepository{Pool: pool}
	idRepo := &postgres.IdentityRepository{Pool: pool}

	// Initialisation des UseCases
	registerUC := &usecase.RegisterResourceTypeUseCase{Repo: rtRepo, PermissionRepo: pRepo}
	createUC := &usecase.CreateResourceUseCase{
		Pool:             pool,
		ResourceRepo:     rRepo,
		ResourceTypeRepo: rtRepo,
		EventRepo:        eRepo,
		PermissionRepo:   pRepo,
		EventPublisher:   nil,
	}
	transitionUC := &usecase.TransitionResourceUseCase{
		ResourceRepo:     rRepo,
		ResourceTypeRepo: rtRepo,
		EventRepo:        eRepo,
		PermissionRepo:   pRepo,
		EventPublisher:   nil,
		Logger:           zerolog.Nop(),
	}

	// 1. Setup Identity & Permissions
	adminID := uuid.New()
	err := idRepo.Create(ctx, &identity.Identity{
		ID: adminID, ExternalID: "admin-test", Name: "Admin Test", Type: "user", CreatedAt: time.Now(),
	})
	require.NoError(t, err)

	// Injection SQL directe pour les permissions (plus simple pour le test d'intégration)
	_, err = pool.Exec(ctx, "INSERT INTO kors.permissions (id, identity_id, action, created_at) VALUES ($1, $2, $3, NOW())", uuid.New(), adminID, "admin")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO kors.permissions (id, identity_id, action, created_at) VALUES ($1, $2, $3, NOW())", uuid.New(), adminID, "write")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO kors.permissions (id, identity_id, action, created_at) VALUES ($1, $2, $3, NOW())", uuid.New(), adminID, "transition")
	require.NoError(t, err)

	// 2. Enregistrement du Type de Ressource
	typeName := "cnc_machine"
	rt, err := registerUC.Execute(ctx, usecase.RegisterResourceTypeInput{
		Name:        typeName,
		Description: "Machine à commande numérique",
		IdentityID:  adminID,
		JSONSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"power_consumption": map[string]interface{}{"type": "number"},
				"operator":          map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"operator"},
		},
		Transitions: map[string]interface{}{
			"off":     []interface{}{"idle"},
			"idle":    []interface{}{"running", "off"},
			"running": []interface{}{"idle", "emergency_stop"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, typeName, rt.Name)

	// 3. Création d'une ressource
	res, err := createUC.Execute(ctx, usecase.CreateResourceInput{
		TypeName:     typeName,
		InitialState: "off",
		IdentityID:   adminID,
		Metadata: map[string]interface{}{
			"operator":          "Jean-Paul",
			"power_consumption": 12.5,
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "off", res.State)

	// Test de validation du schéma (doit échouer)
	_, err = createUC.Execute(ctx, usecase.CreateResourceInput{
		TypeName:     typeName,
		InitialState: "off",
		IdentityID:   adminID,
		Metadata: map[string]interface{}{
			"power_consumption": 100,
		},
	})
	assert.Error(t, err)

	// 4. Transition d'état VALIDE (off -> idle)
	resUpdated, err := transitionUC.Execute(ctx, usecase.TransitionResourceInput{
		ResourceID: res.ID,
		ToState:    "idle",
		IdentityID: adminID,
		Metadata: map[string]interface{}{
			"power_consumption": 1.2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "idle", resUpdated.State)

	// 5. Transition d'état INVALIDE (idle -> emergency_stop n'est pas permis directement)
	_, err = transitionUC.Execute(ctx, usecase.TransitionResourceInput{
		ResourceID: res.ID,
		ToState:    "emergency_stop",
		IdentityID: adminID,
	})
	assert.Error(t, err)

	// 6. Vérification des événements (Audit Trail)
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM kors.events WHERE resource_id = $1", res.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "Should have 2 events: creation and state_changed")
}
