-- name: GetUserByUsername :one

SELECT id, username, password_hash, created_at
FROM users
WHERE username = $1;

-- name: CreateUser :one

INSERT INTO users(
    id,
    username,
    password_hash
)
VALUES ($1, $2, $3)
RETURNING id, username, password_hash, created_at;