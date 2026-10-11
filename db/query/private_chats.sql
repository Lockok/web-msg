-- name: GetPrivateChatByPair :one

SELECT c.id, c.type, c.name, c.created_at
FROM chats AS c
JOIN private_chat_pairs AS p ON p.chat_id = c.id
WHERE p.user_low_id = sqlc.arg(user_low_id)
  AND p.user_high_id = sqlc.arg(user_high_id)
  AND c.type = 'private';


-- name: CreatePrivateChat :one

INSERT INTO chats (id, type)
VALUES (sqlc.arg(id), 'private')
RETURNING id, type, name, created_at;


-- name: CreatePrivateChatPair :exec

INSERT INTO private_chat_pairs (
    chat_id,
    user_low_id,
    user_high_id
)
VALUES (
    sqlc.arg(chat_id),
    sqlc.arg(user_low_id),
    sqlc.arg(user_high_id)
);


-- name: AddPrivateChatMembers :exec

INSERT INTO chat_members (chat_id, user_id, role)
VALUES
    (
        sqlc.arg(chat_id),
        sqlc.arg(user_low_id),
        'member'
    ),
    (
        sqlc.arg(chat_id),
        sqlc.arg(user_high_id),
        'member'
    );