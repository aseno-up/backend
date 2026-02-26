package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aseno/backend/internal/errors"
	"github.com/aseno/backend/pkg/whatsapp/model"
)

// ---------------------------------------------------------------------------
// InMemory stub (implementa WhatsAppConnectionRepository sem banco)
// ---------------------------------------------------------------------------

// inMemoryConnRepo é um stub em memória da interface WhatsAppConnectionRepository.
// Usado em testes de camadas superiores (service, handler) sem precisar de banco.
type inMemoryConnRepo struct {
	mu    sync.RWMutex
	store map[uuid.UUID]*model.WhatsAppConnection // keyed by ID
}

func newInMemoryConnRepo() *inMemoryConnRepo {
	return &inMemoryConnRepo{
		store: make(map[uuid.UUID]*model.WhatsAppConnection),
	}
}

func (r *inMemoryConnRepo) Create(_ context.Context, conn *model.WhatsAppConnection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if conn.ID == uuid.Nil {
		conn.ID = uuid.New()
	}
	cp := *conn
	r.store[conn.ID] = &cp
	return nil
}

func (r *inMemoryConnRepo) GetActiveByClinicID(_ context.Context, clinicID uuid.UUID) (*model.WhatsAppConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.store {
		if c.ClinicID == clinicID && c.IsActive {
			cp := *c
			return &cp, nil
		}
	}
	return nil, errors.ErrNotFoundInstance.WithMessage("WhatsApp connection not found")
}

func (r *inMemoryConnRepo) GetByInstance(_ context.Context, instance string) (*model.WhatsAppConnection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.store {
		if c.Instance == instance {
			cp := *c
			return &cp, nil
		}
	}
	return nil, errors.ErrNotFoundInstance.WithMessage("WhatsApp connection not found")
}

func (r *inMemoryConnRepo) UpdateConnected(_ context.Context, clinicID uuid.UUID, connected bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.store {
		if c.ClinicID == clinicID && c.IsActive {
			c.Connected = connected
			return nil
		}
	}
	return errors.ErrNotFoundInstance.WithMessage("WhatsApp connection not found")
}

func (r *inMemoryConnRepo) UpsertInstance(ctx context.Context, clinicID uuid.UUID, instance string) error {
	existing, err := r.GetActiveByClinicID(ctx, clinicID)
	if err != nil {
		return r.Create(ctx, &model.WhatsAppConnection{
			ClinicID:  clinicID,
			Instance:  instance,
			Connected: false,
			IsActive:  true,
		})
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[existing.ID].Instance = instance
	return nil
}

// ---------------------------------------------------------------------------
// Testes unitários da interface (usando o stub inMemory)
// ---------------------------------------------------------------------------

// TestConnRepo_Create verifica que Create persiste e o ID é gerado.
func TestConnRepo_Create(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()
	clinicID := uuid.New()

	conn := &model.WhatsAppConnection{
		ClinicID:  clinicID,
		Instance:  "test-instance",
		Connected: false,
		IsActive:  true,
	}

	err := repo.Create(ctx, conn)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, conn.ID, "ID deve ser gerado automaticamente")
}

// TestConnRepo_GetActiveByClinicID_Exists retorna a conexão ativa da clínica.
func TestConnRepo_GetActiveByClinicID_Exists(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()
	clinicID := uuid.New()

	require.NoError(t, repo.Create(ctx, &model.WhatsAppConnection{
		ClinicID: clinicID,
		Instance: "minha-clinica-abc12345",
		IsActive: true,
	}))

	got, err := repo.GetActiveByClinicID(ctx, clinicID)
	require.NoError(t, err)
	assert.Equal(t, clinicID, got.ClinicID)
	assert.Equal(t, "minha-clinica-abc12345", got.Instance)
}

// TestConnRepo_GetActiveByClinicID_NotFound retorna erro quando não existe conexão ativa.
func TestConnRepo_GetActiveByClinicID_NotFound(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()

	_, err := repo.GetActiveByClinicID(ctx, uuid.New())
	assert.Error(t, err, "deve retornar erro quando não há conexão ativa")
}

// TestConnRepo_GetActiveByClinicID_InactiveIgnored verifica invariante MVP:
// apenas conexões com IsActive=true são retornadas.
func TestConnRepo_GetActiveByClinicID_InactiveIgnored(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()
	clinicID := uuid.New()

	// Cria conexão inativa (histórico)
	require.NoError(t, repo.Create(ctx, &model.WhatsAppConnection{
		ClinicID: clinicID,
		Instance: "instance-velha",
		IsActive: false,
	}))

	_, err := repo.GetActiveByClinicID(ctx, clinicID)
	assert.Error(t, err, "conexão inativa não deve ser retornada como ativa")
}

// TestConnRepo_GetByInstance_Exists retorna conexão pelo nome da instance.
func TestConnRepo_GetByInstance_Exists(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()
	instance := "clinica-sp-a1b2c3d4"
	clinicID := uuid.New()

	require.NoError(t, repo.Create(ctx, &model.WhatsAppConnection{
		ClinicID: clinicID,
		Instance: instance,
		IsActive: true,
	}))

	got, err := repo.GetByInstance(ctx, instance)
	require.NoError(t, err)
	assert.Equal(t, instance, got.Instance)
	assert.Equal(t, clinicID, got.ClinicID)
}

// TestConnRepo_GetByInstance_NotFound retorna erro para instance inexistente.
// Crítico: um webhook com instance desconhecida deve resultar em erro, não panic.
func TestConnRepo_GetByInstance_NotFound(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()

	_, err := repo.GetByInstance(ctx, "instance-inexistente")
	assert.Error(t, err)
}

// TestConnRepo_UpdateConnected_ToTrue atualiza status de Connected para true.
func TestConnRepo_UpdateConnected_ToTrue(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()
	clinicID := uuid.New()

	require.NoError(t, repo.Create(ctx, &model.WhatsAppConnection{
		ClinicID:  clinicID,
		Instance:  "test-instance",
		Connected: false,
		IsActive:  true,
	}))

	err := repo.UpdateConnected(ctx, clinicID, true)
	require.NoError(t, err)

	got, err := repo.GetActiveByClinicID(ctx, clinicID)
	require.NoError(t, err)
	assert.True(t, got.Connected)
}

// TestConnRepo_UpdateConnected_ToFalse verifica que desconexão é refletida.
func TestConnRepo_UpdateConnected_ToFalse(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()
	clinicID := uuid.New()

	require.NoError(t, repo.Create(ctx, &model.WhatsAppConnection{
		ClinicID:  clinicID,
		Instance:  "test-instance",
		Connected: true,
		IsActive:  true,
	}))

	err := repo.UpdateConnected(ctx, clinicID, false)
	require.NoError(t, err)

	got, err := repo.GetActiveByClinicID(ctx, clinicID)
	require.NoError(t, err)
	assert.False(t, got.Connected)
}

// TestConnRepo_UpdateConnected_NotFound retorna erro para clínica sem conexão ativa.
func TestConnRepo_UpdateConnected_NotFound(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()

	err := repo.UpdateConnected(ctx, uuid.New(), true)
	assert.Error(t, err)
}

// TestConnRepo_UpsertInstance_Cria_Quando_Inexistente garante que UpsertInstance
// cria uma nova entrada quando não existe nenhuma conexão ativa.
func TestConnRepo_UpsertInstance_Cria_Quando_Inexistente(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()
	clinicID := uuid.New()
	instance := "clinica-nova-xyz12345"

	err := repo.UpsertInstance(ctx, clinicID, instance)
	require.NoError(t, err)

	got, err := repo.GetActiveByClinicID(ctx, clinicID)
	require.NoError(t, err)
	assert.Equal(t, instance, got.Instance)
	assert.True(t, got.IsActive)
	assert.False(t, got.Connected, "nova conexão deve iniciar desconectada")
}

// TestConnRepo_UpsertInstance_Atualiza_Quando_Existente atualiza o nome da instance
// quando já existe uma conexão ativa — MVP: uma clínica, uma instance.
func TestConnRepo_UpsertInstance_Atualiza_Quando_Existente(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()
	clinicID := uuid.New()

	require.NoError(t, repo.Create(ctx, &model.WhatsAppConnection{
		ClinicID:  clinicID,
		Instance:  "instance-antiga",
		Connected: true,
		IsActive:  true,
	}))

	err := repo.UpsertInstance(ctx, clinicID, "instance-nova")
	require.NoError(t, err)

	got, err := repo.GetActiveByClinicID(ctx, clinicID)
	require.NoError(t, err)
	assert.Equal(t, "instance-nova", got.Instance, "instance deve ter sido atualizada")
}

// TestConnRepo_MultiTenantIsolation verifica que conexões de clínicas
// diferentes são completamente isoladas — INV-001 (multi-tenancy).
func TestConnRepo_MultiTenantIsolation(t *testing.T) {
	repo := newInMemoryConnRepo()
	ctx := context.Background()

	clinicA := uuid.New()
	clinicB := uuid.New()

	require.NoError(t, repo.Create(ctx, &model.WhatsAppConnection{
		ClinicID: clinicA,
		Instance: "instance-clinica-a",
		IsActive: true,
	}))
	require.NoError(t, repo.Create(ctx, &model.WhatsAppConnection{
		ClinicID: clinicB,
		Instance: "instance-clinica-b",
		IsActive: true,
	}))

	// Atualizar clínica A não afeta clínica B
	require.NoError(t, repo.UpdateConnected(ctx, clinicA, true))

	connA, err := repo.GetActiveByClinicID(ctx, clinicA)
	require.NoError(t, err)
	assert.True(t, connA.Connected)

	connB, err := repo.GetActiveByClinicID(ctx, clinicB)
	require.NoError(t, err)
	assert.False(t, connB.Connected, "clínica B não deve ser afetada pela atualização de A")
}
