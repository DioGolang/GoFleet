CREATE TABLE outbox (
                        id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                        aggregate_type VARCHAR(255) NOT NULL, -- ex: "Order"
                        aggregate_id   VARCHAR(255) NOT NULL, -- ex: order_id
                        event_type     VARCHAR(255) NOT NULL, -- ex: "OrderCreated"
                        event_version  INT NOT NULL DEFAULT 1,
                        payload        JSONB NOT NULL,        -- O evento em si
                        topic          VARCHAR(255) NOT NULL, -- Routing Key do RabbitMQ
                        status       VARCHAR(20) NOT NULL DEFAULT 'PENDING',
                        CONSTRAINT outbox_status_check
                            CHECK (status IN('PENDING', 'PROCESSING', 'PUBLISHED', 'FAILED')),
                        retry_count  INT NOT NULL DEFAULT 0,
                        tracing_context JSONB NOT NULL DEFAULT '{}'::jsonb,
                        error_msg    TEXT,
                        created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
                        updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
                        published_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_outbox_fetch_pending
    ON outbox(created_at)
    WHERE status = 'PENDING';

CREATE INDEX idx_outbox_rescue_processing
    ON outbox(updated_at)
    WHERE status = 'PROCESSING';

CREATE INDEX idx_outbox_cleanup
    ON outbox(created_at)
    WHERE status IN ('PUBLISHED', 'FAILED');

CREATE INDEX idx_outbox_aggregate
    ON outbox(aggregate_type, aggregate_id);