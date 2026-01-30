CREATE TABLE outbox (
                        id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                        aggregate_type VARCHAR(255) NOT NULL, -- ex: "Order"
                        aggregate_id   VARCHAR(255) NOT NULL, -- ex: order_id
                        event_type     VARCHAR(255) NOT NULL, -- ex: "OrderCreated"
                        payload        JSONB NOT NULL,        -- O evento em si
                        topic          VARCHAR(255) NOT NULL, -- Routing Key do RabbitMQ
                        status       VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING, PROCESSING, PUBLISHED, FAILED
                        retry_count  INT NOT NULL DEFAULT 0,
                        error_msg    TEXT,
                        created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
                        updated_at   TIMESTAMP NOT NULL DEFAULT NOW(),
                        published_at TIMESTAMP
);

CREATE INDEX idx_outbox_pending_processing
    ON outbox(created_at)
    WHERE status IN ('PENDING', 'PROCESSING');

CREATE INDEX idx_outbox_cleanup
    ON outbox(status, created_at)
    WHERE status IN ('PUBLISHED', 'FAILED');