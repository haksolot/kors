# ADR-009 — BFF REST + WebSocket Gateway

| Field | Value |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-03-28 |
| **Deciders** | Architecture lead |
| **Applies to** | `services/bff/` |

---

## Context

MES et QMS sont des microservices internes qui communiquent via NATS Request-Reply + Protobuf.
Aucun client (navigateur, tablette atelier) ne peut parler directement à NATS.
Il faut une passerelle qui :

- Expose une API REST JSON sur HTTP pour les clients
- Pousse les events en temps réel via WebSocket (tablettes atelier = mise à jour live)
- Valide les JWT Keycloak sur chaque requête
- Propage l'`operator_id` (Subject JWT) dans chaque appel NATS

## Décisions

### 1. HTTP framework — `chi`

`chi` v5 : router HTTP minimaliste, pur `net/http`, middleware composable, pas de magic.
Aucune dépendance transitive problématique. Cohérent avec le style Go du projet.

Alternatives rejetées :
- `gin` : trop opinionné, génère du `interface{}` partout, réflexion à l'exécution
- `stdlib` bare : pas de route params (`:id`), trop verbeux pour 40+ endpoints

### 2. WebSocket — `nhooyr.io/websocket`

Moderne, context-aware, minimal. `gorilla/websocket` est archivé depuis 2023.
Le hub WebSocket est un goroutine unique qui gère les connexions et le fan-out des events.

### 3. Sérialisation — `protojson`

Les handlers BFF ne définissent **pas** de structs JSON intermédiaires.
- Request body → `protojson.Unmarshal` → proto request message → NATS
- Proto response ← NATS → `protojson.Marshal` → JSON response

Avantage : zéro duplication, cohérence garantie entre l'API REST et le schéma proto.
Nommage JSON = nommage proto (snake_case).

### 4. Auth — JWT middleware via `core.JWTValidator`

`core.JWTValidator` (déjà dans `libs/core`) valide les tokens Keycloak via JWKS.
Le middleware extrait `claims.Subject` (UUID Keycloak) → injecté en contexte.
Les handlers récupèrent le Subject pour le remplir dans `operator_id` des requêtes NATS.

WebSocket : token passé en query param `?token=<jwt>` (browser WebSocket API
n'autorise pas les headers custom à l'upgrade).

### 5. Structure du module

```
services/bff/
  go.mod
  cmd/
    main.go             # bootstrap, config, graceful shutdown
  handler/
    handler.go          # struct Handler + New(), routes chi
    middleware.go       # JWT auth, logging, Prometheus HTTP metrics
    mes.go              # 30 handlers MES (REST)
    qms.go              # 10 handlers QMS (REST)
    ws.go               # Hub WebSocket + handler /ws
    mes_test.go
    qms_test.go
```

---

## API Surface complète

### MES — Manufacturing Execution

```
POST   /api/v1/orders                                      CreateOrder
GET    /api/v1/orders                                      ListOrders
GET    /api/v1/orders/:id                                  GetOrder
POST   /api/v1/orders/:id/suspend                          SuspendOrder
POST   /api/v1/orders/:id/resume                           ResumeOrder
POST   /api/v1/orders/:id/cancel                           CancelOrder
POST   /api/v1/orders/:id/approve-fai                      ApproveFAI
PATCH  /api/v1/orders/:id/planning                         SetPlanning
GET    /api/v1/dispatch                                    GetDispatchList

POST   /api/v1/routings                                    CreateRouting
GET    /api/v1/routings                                    ListRoutings
GET    /api/v1/routings/:id                                GetRouting
POST   /api/v1/orders/from-routing                         CreateFromRouting

GET    /api/v1/orders/:id/operations                       ListOperations
GET    /api/v1/orders/:id/operations/:op_id               GetOperation
POST   /api/v1/orders/:id/operations/:op_id/start         StartOperation
POST   /api/v1/orders/:id/operations/:op_id/complete      CompleteOperation
POST   /api/v1/orders/:id/operations/:op_id/skip          SkipOperation
POST   /api/v1/orders/:id/operations/:op_id/sign-off      SignOffOperation
POST   /api/v1/orders/:id/operations/:op_id/nc            DeclareNC
POST   /api/v1/orders/:id/operations/:op_id/instructions  AttachInstructions

POST   /api/v1/lots                                       CreateLot
GET    /api/v1/lots/:id                                   GetLot

POST   /api/v1/serial-numbers                             RegisterSN
GET    /api/v1/serial-numbers/:sn                         GetSN
POST   /api/v1/serial-numbers/:sn/release                 ReleaseSN
POST   /api/v1/serial-numbers/:sn/scrap                   ScrapSN
POST   /api/v1/serial-numbers/:sn/genealogy               AddGenealogyEntry
GET    /api/v1/serial-numbers/:sn/genealogy               GetGenealogy
```

### QMS — Quality Management

```
GET    /api/v1/qms/nc                                     ListNCs
GET    /api/v1/qms/nc/:id                                 GetNC
POST   /api/v1/qms/nc/:id/analyse                         StartAnalysis
POST   /api/v1/qms/nc/:id/disposition                     ProposeDisposition
POST   /api/v1/qms/nc/:id/close                           CloseNC

POST   /api/v1/qms/capa                                   CreateCAPA
GET    /api/v1/qms/capa                                   ListCAPAs
GET    /api/v1/qms/capa/:id                               GetCAPA
POST   /api/v1/qms/capa/:id/start                         StartCAPA
POST   /api/v1/qms/capa/:id/complete                      CompleteCAPA
```

### Infrastructure

```
GET    /health                                            Health check (200 OK)
GET    /metrics                                           Prometheus metrics
GET    /ws?token=<jwt>                                    WebSocket upgrade
```

---

## WebSocket — Design du Hub

Le Hub est un goroutine singleton qui :
1. Reçoit les events des workers NATS (via channel interne)
2. Les fan-out à tous les clients connectés dont le rôle est autorisé

### Events pushés et filtrage par rôle

| Subject NATS | Rôles autorisés |
|---|---|
| `kors.mes.of.created` | tous |
| `kors.mes.of.suspended` | tous |
| `kors.mes.of.resumed` | tous |
| `kors.mes.of.cancelled` | tous |
| `kors.mes.operation.started` | tous |
| `kors.mes.operation.completed` | tous |
| `kors.mes.nc.declared` | `kors-quality`, `kors-admin` |
| `kors.qms.nc.opened` | `kors-quality`, `kors-admin` |
| `kors.qms.nc.closed` | `kors-quality`, `kors-admin` |
| `kors.qms.capa.created` | `kors-quality`, `kors-admin` |

### Format message WebSocket (JSON)

```json
{
  "type": "kors.mes.of.created",
  "payload": { /* protojson du message proto correspondant */ }
}
```

### Flow auth WebSocket

```
Client → GET /ws?token=<jwt>
BFF    → ValidateJWT(token) → Claims{Subject, Roles}
BFF    → Upgrade WebSocket, enregistre le client avec ses rôles
Hub    → Fan-out uniquement les events autorisés pour ces rôles
```

---

## Convention HTTP → NATS

Chaque handler suit le même pattern :

```go
func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
    claims := claimsFromCtx(r.Context())          // injecté par middleware JWT

    var req pbmes.CreateOrderRequest
    if err := unmarshalBody(r, &req); err != nil { // protojson.Unmarshal
        writeError(w, http.StatusBadRequest, err)
        return
    }

    var resp pbmes.CreateOrderResponse
    if err := h.natsReq(r.Context(), mesdomain.SubjectOFCreate, &req, &resp); err != nil {
        writeError(w, http.StatusInternalServerError, err)
        return
    }

    writeJSON(w, http.StatusCreated, &resp)         // protojson.Marshal
}
```

`operator_id` est injecté depuis `claims.Subject` dans les handlers qui le nécessitent
(StartOperation, SignOffOperation, DeclareNC, etc.) — jamais depuis le body client.

---

## Config (variables d'environnement)

| Variable | Défaut | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8080` | Adresse d'écoute HTTP |
| `METRICS_ADDR` | `:9092` | Port Prometheus |
| `NATS_URL` | `nats://localhost:4222` | URL NATS |
| `NATS_CREDS_PATH` | — | Chemin fichier credentials NATS (optionnel) |
| `JWKS_ENDPOINT` | — | URL JWKS Keycloak |
| `OTLP_ENDPOINT` | — | Endpoint OpenTelemetry |
| `SERVICE_NAME` | `kors-bff` | Nom du service (traces) |
| `LOG_LEVEL` | `info` | Niveau de log |

---

## Graceful shutdown

```
SIGINT/SIGTERM
  → http.Server.Shutdown(ctx 15s)   # drain des connexions HTTP
  → Hub.Close()                      # ferme toutes les connexions WS
  → nc.Drain()                       # attendre la fin des subs NATS
```

---

## Conséquences

**Positives :**
- API REST JSON standard, consommable par n'importe quel client (navigateur, Postman, curl)
- Zéro duplication grâce à `protojson` — un seul schéma de données
- WebSocket avec filtrage rôle = tablettes opérateur ne voient que ce qui les concerne
- JWT validé côté serveur, `operator_id` jamais truquable depuis le client
- Cohérent avec ADR-006 (no gRPC), ADR-008 (OTel), ADR-002 (NATS)

**Contraintes :**
- Le BFF est un SPOF léger — mitigation : déployer 2 réplicas derrière un LB (k3s, ADR-005)
- `protojson` expose les noms de champs proto — un refactor proto = breaking change API
- WebSocket token en query param peut apparaître dans les logs HTTP — mitigation : log masqué dans le middleware

## Règles pour les agents

```
NEVER: accepter operator_id depuis le body HTTP — toujours depuis claims.Subject
NEVER: définir des structs JSON intermédiaires — utiliser protojson directement
NEVER: appeler les services MES/QMS autrement que via nc.Request() NATS
ALWAYS: valider le JWT sur CHAQUE endpoint, y compris WebSocket
ALWAYS: injecter claims dans le context, jamais en variable globale
ALWAYS: retourner les erreurs NATS comme 502 Bad Gateway (service interne)
ALWAYS: retourner les erreurs de validation JSON comme 400 Bad Request
```

## Related ADRs

- ADR-002 : NATS (transport interne)
- ADR-006 : No gRPC (BFF = seul point d'exposition REST)
- ADR-008 : Observability (OTel traces propagées BFF→MES/QMS)
