-- name: SearchChatMessages :many
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
  AND strpos(lower(msg.text), lower(sqlc.arg(search_text)::text)) > 0
  AND (
      sqlc.arg(before_seq)::bigint = 0
      OR msg.seq < sqlc.arg(before_seq)::bigint
  )
ORDER BY msg.seq DESC
LIMIT sqlc.arg(page_size)::integer;