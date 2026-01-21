-- name: CreateOrder :exec
INSERT INTO orders (id, price, tax, final_price, status, driver_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetOrder :one
SELECT id, price, tax, final_price, status, driver_id FROM orders
WHERE id = $1;

-- name: ListOrders :many
SELECT id, price, tax, final_price FROM orders;

-- name: UpdateOrderStatus :exec
UPDATE orders
SET status = $1, driver_id = $2
WHERE id = $3;

