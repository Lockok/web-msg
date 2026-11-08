-- name: ListChatMessagesAfter :many
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
JOIN chat_members AS member
    ON member.chat_id = msg.chat_id
LEFT JOIN chat_history_clearances AS clearance
    ON clearance.chat_id = msg.chat_id
    AND clearance.user_id = member.user_id
WHERE msg.chat_id = sqlc.arg(chat_id)
    AND member.user_id = sqlc.arg(user_id)
    AND msg.seq > sqlc.arg(after_seq)::bigint
    AND msg.seq > COALESCE(clearance.cleared_through_seq, 0)
ORDER BY msg.seq ASC
LIMIT sqlc.arg(page_size)::integer;