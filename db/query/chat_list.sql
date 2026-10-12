-- name: ListUserChats :many

SELECT c.id, c.type, c.name, c.created_at
FROM chats AS c
JOIN chat_members AS m ON m.chat_id = c.id
WHERE m.user_id = sqlc.arg(user_id)
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(page_size)::integer;


-- name: ListUserChatsBefore :many

SELECT c.id, c.type, c.name, c.created_at
FROM chats AS c
JOIN chat_members AS m ON m.chat_id = c.id
WHERE m.user_id = sqlc.arg(user_id)
  AND (c.created_at, c.id) < (
      sqlc.arg(cursor_created_at)::timestamptz,
      sqlc.arg(cursor_id)::uuid
  )
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(page_size)::integer;