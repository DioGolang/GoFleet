package event

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/DioGolang/GoFleet/internal/infra/database"
	"github.com/DioGolang/GoFleet/pkg/events"
	"github.com/DioGolang/GoFleet/pkg/logger"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type OutboxRelay struct {
	db         *database.Queries
	dbConn     *sql.DB
	dispatcher events.EventDispatcher
	logger     logger.Logger
	batchSize  int32
	workers    int
}

func NewOutboxRelay(db *database.Queries, conn *sql.DB, disp events.EventDispatcher, log logger.Logger) *OutboxRelay {
	return &OutboxRelay{
		db:         db,
		dbConn:     conn,
		dispatcher: disp,
		logger:     log,
		batchSize:  100,
		workers:    10,
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
	// FASE 1: Fetch & Claim (Transação Curta)
	eventsToProcess, err := r.fetchAndClaim(ctx)
	if err != nil {
		if err != sql.ErrNoRows {
			r.logger.Error(ctx, "Failed to fetch batch", logger.WithError(err))
		}
		return
	}

	if len(eventsToProcess) == 0 {
		return
	}

	// FASE 2: Dispatch (Network I/O - Fora da Transação)
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(r.workers)

	for _, evt := range eventsToProcess {
		evt := evt // Capture loop variable
		g.Go(func() error {
			return r.processSingleEvent(gCtx, evt)
		})
	}

	if err := g.Wait(); err != nil {
		r.logger.Error(ctx, "Batch processing had errors", logger.WithError(err))
	}
}

func (r *OutboxRelay) fetchAndClaim(ctx context.Context) ([]database.FetchPendingOutboxEventsRow, error) {
	tx, err := r.dbConn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	qtx := r.db.WithTx(tx)

	events, err := qtx.FetchPendingOutboxEvents(ctx, r.batchSize)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, len(events))
	for i, e := range events {
		ids[i] = e.ID
	}

	if err := qtx.MarkOutboxAsProcessing(ctx, ids); err != nil {
		return nil, err
	}
	return events, tx.Commit()
}

func (r *OutboxRelay) processSingleEvent(ctx context.Context, evt database.FetchPendingOutboxEventsRow) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	versionStr := strconv.FormatInt(int64(evt.EventVersion), 10)
	headers := map[string]string{
		"x-event-version": versionStr,
		"x-event-id":      evt.ID.String(),
		"x-aggregate-id":  evt.AggregateID,
	}

	err := r.dispatcher.DispatchRaw(ctx, evt.Topic, evt.Payload, headers)

	// FASE 3: Atualização de Estado
	if err != nil {
		r.logger.Warn(ctx, "Failed to publish event",
			logger.String("id", evt.ID.String()),
			logger.WithError(err))

		return r.db.MarkOutboxAsFailed(context.Background(), database.MarkOutboxAsFailedParams{
			ID:       evt.ID,
			ErrorMsg: sql.NullString{String: err.Error(), Valid: true},
		})
	}

	return r.db.MarkOutboxAsPublished(context.Background(), evt.ID)
}

func (r *OutboxRelay) RunRescuer(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.db.ResetStuckEvents(ctx, "5 minutes"); err != nil {
				r.logger.Error(ctx, "Failed to reset stuck events", logger.WithError(err))
			}

			if err := r.db.DeleteOldOutboxEvents(ctx, "7 days"); err != nil {
				r.logger.Error(ctx, "Cleanup failed", logger.WithError(err))
			}
		}
	}
}
