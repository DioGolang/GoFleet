-- name: CreateOrder :exec
INSERT INTO orders (id, price, tax, final_price)
VALUES ($1, $2, $3, $4);

-- name: GetOrder :one
SELECT id, price, tax, final_price FROM orders
WHERE id = $1;

-- name: ListOrders :many
SELECT id, price, tax, final_price FROM orders;