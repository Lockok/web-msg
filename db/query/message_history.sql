-- name: ChatExists :one

SELECT EXISTS (
    SELECT 1
    FROM chats
    WHERE id = sqlc.arg(chat_id)
) AS chat_exists;


-- name: ListChatMessages :many

SELECT
    msg.id,
    msg.chat_id,
    msg.sender_id,
    msg.text,
    msg.created_at,
    msg.delivered_at,
    msg.seq,
    msg.client_message_id
FROM messages AS msg
JOIN chat_members AS member ON member.chat_id = msg.chat_id
WHERE msg.chat_id = sqlc.arg(chat_id)
  AND member.user_id = sqlc.arg(user_id)
ORDER BY msg.seq DESC
LIMIT sqlc.arg(page_size)::integer;


-- name: ListChatMessagesBefore :many

SELECT
    msg.id,
    msg.chat_id,
    msg.sender_id,
    msg.text,
    msg.created_at,
    msg.delivered_at,
    msg.seq,
    msg.client_message_id
FROM messages AS msg
JOIN chat_members AS member ON member.chat_id = msg.chat_id
WHERE msg.chat_id = sqlc.arg(chat_id)
  AND member.user_id = sqlc.arg(user_id)
  AND msg.seq < sqlc.arg(before_seq)::bigint
ORDER BY msg.seq DESC
LIMIT sqlc.arg(page_size)::integer;