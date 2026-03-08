package mocks

import (
    "context"
    "github.com/google/uuid"
    "github.com/haksolot/kors/kors-api/internal/domain/event"
    "github.com/haksolot/kors/kors-api/internal/domain/permission"
    "github.com/haksolot/kors/kors-api/internal/domain/resource"
    "github.com/haksolot/kors/kors-api/internal/domain/resourcetype"
)

// --- ResourceTypeRepository mock ---

type ResourceTypeRepo struct {
    Types map[string]*resourcetype.ResourceType
}

func (m *ResourceTypeRepo) Create(_ context.Context, rt *resourcetype.ResourceType) error {
    if m.Types == nil { m.Types = make(map[string]*resourcetype.ResourceType) }
    m.Types[rt.Name] = rt
    return nil
}

func (m *ResourceTypeRepo) GetByName(_ context.Context, name string) (*resourcetype.ResourceType, error) {
    if m.Types == nil { return nil, nil }
    rt, ok := m.Types[name]
    if !ok { return nil, nil }
    return rt, nil
}

func (m *ResourceTypeRepo) GetByID(_ context.Context, id uuid.UUID) (*resourcetype.ResourceType, error) {
    if m.Types == nil { return nil, nil }
    for _, rt := range m.Types {
        if rt.ID == id { return rt, nil }
    }
    return nil, nil
}

func (m *ResourceTypeRepo) List(_ context.Context) ([]*resourcetype.ResourceType, error) {
    result := make([]*resourcetype.ResourceType, 0, len(m.Types))
    for _, rt := range m.Types { result = append(result, rt) }
    return result, nil
}

// --- ResourceRepository mock ---

type ResourceRepo struct {
    Resources map[uuid.UUID]*resource.Resource
    CreateErr error
    UpdateErr error
}

func (m *ResourceRepo) Create(_ context.Context, res *resource.Resource) error {
    if m.CreateErr != nil { return m.CreateErr }
    if m.Resources == nil { m.Resources = make(map[uuid.UUID]*resource.Resource) }
    m.Resources[res.ID] = res
    return nil
}

func (m *ResourceRepo) GetByID(_ context.Context, id uuid.UUID) (*resource.Resource, error) {
    if m.Resources == nil { return nil, nil }
    res, ok := m.Resources[id]
    if !ok { return nil, nil }
    return res, nil
}

func (m *ResourceRepo) Update(_ context.Context, res *resource.Resource) error {
    if m.UpdateErr != nil { return m.UpdateErr }
    if m.Resources == nil { m.Resources = make(map[uuid.UUID]*resource.Resource) }
    m.Resources[res.ID] = res
    return nil
}

func (m *ResourceRepo) List(_ context.Context, first int, after *uuid.UUID, typeName *string) ([]*resource.Resource, bool, int, error) {
    result := make([]*resource.Resource, 0)
    for _, res := range m.Resources { result = append(result, res) }
    return result, false, len(result), nil
}

func (m *ResourceRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
    return nil
}

// --- PermissionRepository mock ---

type PermissionRepo struct {
    // Permet de controler le resultat de Check par action
    Allowed map[string]bool // key: action
    AllowAll bool
}

func (m *PermissionRepo) Create(_ context.Context, p *permission.Permission) error { return nil }
func (m *PermissionRepo) Delete(_ context.Context, id uuid.UUID) error { return nil }
func (m *PermissionRepo) FindForIdentity(_ context.Context, identityID uuid.UUID) ([]*permission.Permission, error) { return nil, nil }

func (m *PermissionRepo) Check(_ context.Context, identityID uuid.UUID, action string, resourceID *uuid.UUID, resourceTypeID *uuid.UUID) (bool, error) {
    if m.AllowAll { return true, nil }
    if m.Allowed != nil {
        return m.Allowed[action], nil
    }
    return false, nil
}

// --- EventRepository mock ---

type EventRepo struct {
    Events []*event.Event
    CreateErr error
}

func (m *EventRepo) Create(_ context.Context, e *event.Event) error {
    if m.CreateErr != nil { return m.CreateErr }
    m.Events = append(m.Events, e)
    return nil
}

// --- EventPublisher mock ---

type EventPublisher struct {
    Published []*event.Event
    PublishErr error
}

func (m *EventPublisher) Publish(_ context.Context, e *event.Event) error {
    if m.PublishErr != nil { return m.PublishErr }
    m.Published = append(m.Published, e)
    return nil
}
