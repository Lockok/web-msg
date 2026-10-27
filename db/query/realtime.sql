-- name: ListChatMemberIDs :many
SELECT user_id
FROM chat_members
WHERE chat_id = sqlc.arg(chat_id)
ORDER BY user_id;