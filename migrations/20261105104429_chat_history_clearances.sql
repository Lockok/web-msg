-- +goose Up

CREATE TABLE chat_history_clearances (
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cleared_through_seq BIGINT NOT NULL DEFAULT 0
        CHECK (cleared_through_seq >= 0),

    PRIMARY KEY (chat_id, user_id)
);

-- +goose Down

DROP TABLE chat_history_clearances;