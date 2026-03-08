package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/haksolot/kors/kors-api/internal/domain/identity"
	"github.com/haksolot/kors/kors-api/internal/domain/permission"
)

func connectDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, url)
}

func connectNATS(url string) (*nats.Conn, nats.JetStreamContext, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err == nil {
		_, _ = js.AddStream(&nats.StreamConfig{Name: "KORS", Subjects: []string{"kors.>"}})
	}
	return nc, js, err
}

func connectMinio(url, ak, sk string, ssl bool) (*minio.Client, error) {
	return minio.New(url, &minio.Options{
		Creds:  credentials.NewStaticV4(ak, sk, ""),
		Secure: ssl,
	})
}

func bootstrapSystemIdentity(ctx context.Context, idRepo identity.Repository, pRepo permission.Repository) error {
	sysID, _ := idRepo.GetByExternalID(ctx, "system")
	if sysID == nil {
		_ = idRepo.Create(ctx, &identity.Identity{ID: uuid.Nil, ExternalID: "system", Name: "System", Type: "system", CreatedAt: time.Now()})
	}
	for _, a := range []string{"write", "transition", "admin"} {
		allowed, _ := pRepo.Check(ctx, uuid.Nil, a, nil, nil)
		if !allowed {
			_ = pRepo.Create(ctx, &permission.Permission{ID: uuid.New(), IdentityID: uuid.Nil, Action: a, CreatedAt: time.Now()})
		}
	}
	return nil
}
