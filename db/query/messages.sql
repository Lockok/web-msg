-- name: LockChatForMessage :one

SELECT id, type, name, created_at, last_message_seq
FROM chats
WHERE id = sqlc.arg(chat_id)
FOR UPDATE;


-- name: IsChatMember :one

SELECT EXISTS (
    SELECT 1
    FROM chat_members
    WHERE chat_id = sqlc.arg(chat_id)
      AND user_id = sqlc.arg(user_id)
) AS is_member;


-- name: GetMessageByClientID :one

SELECT
    id,
    chat_id,
    sender_id,
    text,
    created_at,
    delivered_at,
    seq,
    client_message_id
FROM messages
WHERE chat_id = sqlc.arg(chat_id)
  AND sender_id = sqlc.arg(sender_id)
  AND client_message_id = sqlc.arg(client_message_id);


-- name: IncrementChatMessageSeq :one

UPDATE chats
SET last_message_seq = last_message_seq + 1
WHERE id = sqlc.arg(chat_id)
RETURNING last_message_seq;


-- name: CreateMessage :one

INSERT INTO messages (
    id,
    chat_id,
    sender_id,
    text,
    seq,
    client_message_id
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(chat_id),
    sqlc.arg(sender_id),
    sqlc.arg(text),
    sqlc.arg(seq),
    sqlc.arg(client_message_id)
)
RETURNING
    id,
    chat_id,
    sender_id,
    text,
    created_at,
    delivered_at,
    seq,
    client_message_id;