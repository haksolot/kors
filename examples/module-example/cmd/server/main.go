package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/haksolot/kors/sdk/go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/pressly/goose/v3"
	"github.com/haksolot/kors/examples/module-example/internal/graph/generated"
	"github.com/haksolot/kors/examples/module-example/internal/graph/resolvers"
	"github.com/haksolot/kors/examples/module-example/internal/model"
	"github.com/haksolot/kors/examples/module-example/internal/store"
)

func main() {
	_ = godotenv.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Database
	adminDBURL := os.Getenv("DATABASE_URL")
	
	log.Println("Running module migrations using admin credentials...")
	adminDB, _ := stdlib.OpenDB(*pgxpool.New(context.Background(), adminDBURL).Config().ConnConfig)
	_ = goose.Up(adminDB, "/migrations")
	adminDB.Close()

	// Application connection using module credentials (connectionString from provisionModule)
	moduleDBURL := getEnv("MODULE_DATABASE_URL", adminDBURL)
	pool, _ := pgxpool.New(context.Background(), moduleDBURL)
	defer pool.Close()

	// 2. NATS
	nc, _ := nats.Connect(os.Getenv("NATS_URL"))
	js, _ := nc.JetStream()

	// 3. AUTH: Get Token
	token, _ := getKeycloakToken()
	if token == "" { token = "system" }

	// 4. KORS SDK
	korsClient := sdk.NewClient("http://kors-api:8080/query", token)

	// 5. NATS Listener
	go func() {
		sub, _ := js.PullSubscribe("kors.resource.state_changed", "tms-worker")
		for {
			msgs, err := sub.Fetch(1, nats.MaxWait(1*time.Second))
			if err != nil { continue }
			for _, msg := range msgs {
				handleKorsEvent(ctx, pool, msg)
				msg.Ack()
			}
		}
	}()

	// 6. Bootstrap
	go func() {
		time.Sleep(5 * time.Second)
		bootstrapData(ctx, korsClient, pool)
	}()

	// 7. Server
	resolver := &resolvers.Resolver{Store: &store.ToolStore{Pool: pool}}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	http.Handle("/query", srv)
	http.Handle("/", playground.Handler("TMS Subgraph", "/query"))

	log.Println("TMS Module running on :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getKeycloakToken() (string, error) {
	keycloakURL := "http://kors-sso:8080/realms/kors/protocol/openid-connect/token"
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", "tms-module")
	data.Set("client_secret", "W8kEYFsnSDwzzOTBIedGIlOAzxflktMk")

	resp, err := http.PostForm(keycloakURL, data)
	if err != nil { return "", err }
	defer resp.Body.Close()

	var res struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	return res.AccessToken, nil
}

func bootstrapData(ctx context.Context, client *sdk.Client, pool *pgxpool.Pool) {
	log.Println("KORS: Bootstrapping data...")
	
	_, _ = sdk.RegisterResourceType(ctx, client.GQL(), sdk.RegisterResourceTypeInput{
		Name: "tool",
		Transitions: map[string]interface{}{"idle": []string{"maintenance"}, "maintenance": []string{"idle"}},
		JsonSchema: map[string]interface{}{"type": "object"},
	})

	createResp, err := sdk.CreateResource(ctx, client.GQL(), sdk.CreateResourceInput{
		TypeName: "tool", InitialState: "idle", Metadata: map[string]interface{}{"serial": "REAL-AUTH-V2"},
	})

	if err == nil && createResp.CreateResource.Success {
		id := createResp.CreateResource.Resource.Id
		log.Printf("Tool created: %s", id)
		toolStore := &store.ToolStore{Pool: pool}
		_ = toolStore.Save(ctx, &model.Tool{ID: id, SerialNumber: "REAL-AUTH-V2", Model: "PrecisionX", Diameter: 10.5, Length: 100.0})
		_, _ = sdk.TransitionResource(ctx, client.GQL(), sdk.TransitionResourceInput{ResourceId: id, ToState: "maintenance"})
	}
}

func handleKorsEvent(ctx context.Context, pool *pgxpool.Pool, msg *nats.Msg) {
	var ev struct {
		ResourceID string `json:"ResourceID"`
		Payload    map[string]interface{} `json:"Payload"`
	}
	json.Unmarshal(msg.Data, &ev)
	if ev.Payload["to_state"] == "maintenance" {
		log.Printf("TMS: Maintenance detected for %s", ev.ResourceID)
		_, _ = pool.Exec(ctx, "UPDATE tools SET last_maintenance_at = NOW() WHERE id = $1", ev.ResourceID)
	}
}
