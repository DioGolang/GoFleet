package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
)

type UnitOfWorkImpl struct {
	db *sql.DB
}

func NewUnitOfWork(db *sql.DB) *UnitOfWorkImpl {
	return &UnitOfWorkImpl{db: db}
}

func (u *UnitOfWorkImpl) Do(ctx context.Context, fn func(repo outbound.OrderRepository) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	txRepo := &OrderRepositoryImpl{
		Db:      u.db,
		Queries: New(u.db).WithTx(tx), // SQLC magic
	}

	if err := fn(txRepo); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}
