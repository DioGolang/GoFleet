package event

import (
	"context"
	"database/sql"
	"time"

	"github.com/DioGolang/GoFleet/internal/infra/database"
	"github.com/DioGolang/GoFleet/pkg/logger"
)

type OutboxRelay struct {
	db         *database.Queries
	dbConn     *sql.DB
	dispatcher Dispatcher
	logger     logger.Logger
	batchSize  int32
}

func NewOutboxRelay(db *database.Queries, conn *sql.DB, disp Dispatcher, log logger.Logger) *OutboxRelay {
	return &OutboxRelay{
		db:         db,
		dbConn:     conn,
		dispatcher: disp,
		logger:     log,
		batchSize:  50,
	}
}

func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.processBatch(ctx)
		}
	}
}

func (r *OutboxRelay) processBatch(ctx context.Context) {
	tx, err := r.dbConn.BeginTx(ctx, nil)
	if err != nil {
		r.logger.Error(ctx, "Failed to begin transaction", logger.WithError(err))
		return
	}
	defer tx.Rollback() // Safety

	qtx := r.db.WithTx(tx)

	events, err := qtx.FetchPendingOutboxEvents(ctx, r.batchSize)
	if err != nil {
		if err != sql.ErrNoRows {
			r.logger.Error(ctx, "Failed to fetch outbox", logger.WithError(err))
		}
		return
	}

	if len(events) == 0 {
		return // Nada a fazer
	}

	for _, evt := range events {
		err := r.dispatcher.DispatchRaw(ctx, evt.Topic, evt.Payload)

		if err != nil {
			r.logger.Error(ctx, "Failed to publish event",
				logger.String("id", evt.ID.String()),
				logger.WithError(err))

			if errMark := qtx.MarkOutboxAsFailed(ctx, database.MarkOutboxAsFailedParams{
				ID:       evt.ID,
				ErrorMsg: sql.NullString{String: err.Error(), Valid: true},
			}); errMark != nil {
				r.logger.Error(ctx, "Failed to mark as failed", logger.WithError(errMark))
			}
			continue
		}

		if err := qtx.MarkOutboxAsPublished(ctx, evt.ID); err != nil {
			r.logger.Error(ctx, "Failed to mark as published", logger.WithError(err))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		r.logger.Error(ctx, "Failed to commit outbox batch", logger.WithError(err))
	}
}

func (r *OutboxRelay) RunCleaner(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.db.DeleteOldOutboxEvents(ctx, "7 days"); err != nil {
				r.logger.Error(ctx, "Outbox cleanup failed", logger.WithError(err))
			}
		}
	}
}
