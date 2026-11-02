-- name: CreateGroupChat :one
INSERT INTO chats (
    id,
    type,
    name
)
VALUES (
    sqlc.arg(id),
    'group',
    sqlc.arg(name)::text
)
RETURNING id, type, name, created_at, last_message_seq;

-- name: AddGroupChatCreator :exec
INSERT INTO chat_members (
    chat_id,
    user_id,
    role
)
VALUES (
    sqlc.arg(chat_id),
    sqlc.arg(user_id),
    'admin'
);