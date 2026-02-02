package outbound

import (
	"context"
)

// RepositoryProvider define o contrato para acessar TODOS os repositórios
type RepositoryProvider interface {
	Order() OrderRepository
	// Futuro:
	// Account() AccountRepository
	// Inventory() InventoryRepository
}

// UnitOfWork gerencia a atomicidade.
// A função de callback recebe o Provider já "hidratado" com a transação ativa.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(provider RepositoryProvider) error) error
}
