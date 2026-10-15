-- +goose Up

ALTER TABLE chats
    ADD COLUMN last_message_seq BIGINT NOT NULL DEFAULT 0;

ALTER TABLE chats
    ADD CONSTRAINT chats_last_message_seq_nonnegative
    CHECK (last_message_seq >= 0);

ALTER TABLE messages
    ADD COLUMN seq BIGINT,
    ADD COLUMN client_message_id UUID;

-- Если сообщения уже есть, назначаем им порядок внутри чата.
WITH numbered AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY chat_id
            ORDER BY created_at, id
        ) AS message_seq
    FROM messages
)
UPDATE messages AS m
SET seq = n.message_seq
FROM numbered AS n
WHERE m.id = n.id;

-- Для существующих сообщений используем их ID как ключ повторной отправки.
UPDATE messages
SET client_message_id = id;

UPDATE chats AS c
SET last_message_seq = COALESCE(
    (
        SELECT MAX(m.seq)
        FROM messages AS m
        WHERE m.chat_id = c.id
    ),
    0
);

ALTER TABLE messages
    ALTER COLUMN seq SET NOT NULL,
    ALTER COLUMN client_message_id SET NOT NULL;

ALTER TABLE messages
    ADD CONSTRAINT messages_seq_positive
    CHECK (seq > 0);

ALTER TABLE messages
    ADD CONSTRAINT messages_chat_seq_unique
    UNIQUE (chat_id, seq);

ALTER TABLE messages
    ADD CONSTRAINT messages_client_id_unique
    UNIQUE (chat_id, sender_id, client_message_id);

-- +goose Down

ALTER TABLE messages
    DROP CONSTRAINT messages_client_id_unique,
    DROP CONSTRAINT messages_chat_seq_unique,
    DROP CONSTRAINT messages_seq_positive;

ALTER TABLE messages
    DROP COLUMN client_message_id,
    DROP COLUMN seq;

ALTER TABLE chats
    DROP CONSTRAINT chats_last_message_seq_nonnegative,
    DROP COLUMN last_message_seq;