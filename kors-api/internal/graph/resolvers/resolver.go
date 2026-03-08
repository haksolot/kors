package resolvers

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go"
	"github.com/haksolot/kors/kors-api/internal/adapter/postgres"
	korsnats "github.com/haksolot/kors/kors-api/internal/adapter/nats"
	korsminio "github.com/haksolot/kors/kors-api/internal/adapter/minio"
	"github.com/haksolot/kors/kors-api/internal/usecase"
)

type Resolver struct {
	RegisterResourceTypeUseCase *usecase.RegisterResourceTypeUseCase
	CreateResourceUseCase       *usecase.CreateResourceUseCase
	TransitionResourceUseCase   *usecase.TransitionResourceUseCase
	GrantPermissionUseCase      *usecase.GrantPermissionUseCase
	CreateRevisionUseCase       *usecase.CreateRevisionUseCase
	ListResourcesUseCase        *usecase.ListResourcesUseCase
	ModuleGovernanceUseCase     *usecase.ModuleGovernanceUseCase
	NatsConn                    *nats.Conn
}

func NewResolver(pool *pgxpool.Pool, nc *nats.Conn, js nats.JetStreamContext, mClient *minio.Client) *Resolver {
	rtRepo := &postgres.ResourceTypeRepository{Pool: pool}
	rRepo := &postgres.ResourceRepository{Pool: pool}
	eRepo := &postgres.EventRepository{Pool: pool}
	pRepo := &postgres.PermissionRepository{Pool: pool}
	revRepo := &postgres.RevisionRepository{Pool: pool}
	prov := &postgres.PostgresProvisioner{Pool: pool}
	fStore := &korsminio.MinioFileStore{Client: mClient, Bucket: "kors-files"}
	
	var ePub *korsnats.NatsPublisher
	if js != nil { ePub = &korsnats.NatsPublisher{JS: js} }

	return &Resolver{
		NatsConn: nc,
		RegisterResourceTypeUseCase: &usecase.RegisterResourceTypeUseCase{Repo: rtRepo, PermissionRepo: pRepo},
		CreateResourceUseCase:       &usecase.CreateResourceUseCase{ResourceRepo: rRepo, ResourceTypeRepo: rtRepo, EventRepo: eRepo, PermissionRepo: pRepo, EventPublisher: ePub},
		TransitionResourceUseCase:   &usecase.TransitionResourceUseCase{ResourceRepo: rRepo, ResourceTypeRepo: rtRepo, EventRepo: eRepo, PermissionRepo: pRepo, EventPublisher: ePub},
		GrantPermissionUseCase:      &usecase.GrantPermissionUseCase{Repo: pRepo},
		CreateRevisionUseCase:       &usecase.CreateRevisionUseCase{ResourceRepo: rRepo, RevisionRepo: revRepo, FileStore: fStore, EventRepo: eRepo, EventPublisher: ePub},
		ListResourcesUseCase:        &usecase.ListResourcesUseCase{Repo: rRepo},
		ModuleGovernanceUseCase:     &usecase.ModuleGovernanceUseCase{Provisioner: prov, PermissionRepo: pRepo},
	}
}
