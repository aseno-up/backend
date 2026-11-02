package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestWhatsAppConnection_TableName garante que o nome da tabela está correto.
// Crítico: uma migração errada sobrescreve a tabela errada.
func TestWhatsAppConnection_TableName(t *testing.T) {
	conn := WhatsAppConnection{}
	assert.Equal(t, "whatsapp_connections", conn.TableName())
}

// TestWhatsAppConnection_Defaults verifica os valores zero da struct.
func TestWhatsAppConnection_Defaults(t *testing.T) {
	conn := WhatsAppConnection{}
	assert.Equal(t, uuid.Nil, conn.ID)
	assert.Equal(t, uuid.Nil, conn.ClinicID)
	assert.Nil(t, conn.Instance)
	assert.False(t, conn.Connected, "Connected deve ser false por padrão")
	assert.False(t, conn.IsActive, "IsActive deve ser false por padrão")
}

func ptr[T any](v T) *T { return &v }

// TestWhatsAppConnection_Fields verifica que os campos são corretamente atribuíveis.
func TestWhatsAppConnection_Fields(t *testing.T) {
	clinicID := uuid.New()
	now := time.Now()

	conn := WhatsAppConnection{
		ID:        uuid.New(),
		ClinicID:  clinicID,
		Instance:  ptr("minha-clinica-abc12345"),
		Connected: true,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, clinicID, conn.ClinicID)
	assert.Equal(t, "minha-clinica-abc12345", *conn.Instance)
	assert.True(t, conn.Connected)
	assert.True(t, conn.IsActive)
}

// TestWhatsAppConnection_IsActiveSemantics verifica que is_active distingue
// conexões ativas de inativas — invariante MVP (1 ativa por clínica).
func TestWhatsAppConnection_IsActiveSemantics(t *testing.T) {
	clinicID := uuid.New()

	// Conexão ativa
	active := WhatsAppConnection{
		ID:       uuid.New(),
		ClinicID: clinicID,
		Instance: ptr("instance-ativa"),
		IsActive: true,
	}

	// Conexão inativa (histórico)
	inactive := WhatsAppConnection{
		ID:       uuid.New(),
		ClinicID: clinicID,
		Instance: ptr("instance-velha"),
		IsActive: false,
	}

	assert.True(t, active.IsActive)
	assert.False(t, inactive.IsActive)
	// Mesma clínica, instâncias diferentes — sem conflito por design
	assert.Equal(t, clinicID, active.ClinicID)
	assert.Equal(t, clinicID, inactive.ClinicID)
}

// TestWhatsAppConnection_ConnectedDisconnected valida transição de estado.
// Invariante: um webhook de disconnect deve poder setar Connected=false.
func TestWhatsAppConnection_ConnectedDisconnected(t *testing.T) {
	conn := WhatsAppConnection{
		ID:        uuid.New(),
		ClinicID:  uuid.New(),
		Instance:  ptr("test-instance"),
		Connected: true,
		IsActive:  true,
	}

	// Simula desconexão
	conn.Connected = false
	assert.False(t, conn.Connected, "após desconexão Connected deve ser false")
	assert.True(t, conn.IsActive, "IsActive não muda na desconexão")
}
