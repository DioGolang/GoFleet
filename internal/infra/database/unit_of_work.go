package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
)

type RepositoryProviderImpl struct {
	db      *sql.DB
	queries *Queries
}

func (p *RepositoryProviderImpl) Order() outbound.OrderRepository {
	return &OrderRepositoryImpl{
		Db:      p.db,
		Queries: p.queries,
	}
}

type UnitOfWorkImpl struct {
	db *sql.DB
}

func NewUnitOfWork(db *sql.DB) *UnitOfWorkImpl {
	return &UnitOfWorkImpl{db: db}
}

func (u *UnitOfWorkImpl) Do(ctx context.Context, fn func(provider outbound.RepositoryProvider) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	provider := &RepositoryProviderImpl{
		db:      u.db,
		queries: New(u.db).WithTx(tx), // SQLC magic
	}

	if err := fn(provider); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}
