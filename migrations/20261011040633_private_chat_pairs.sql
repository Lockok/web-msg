-- +goose Up
    CREATE TABLE private_chat_pairs (
        chat_id UUID PRIMARY KEY
            REFERENCES chats(id) ON DELETE CASCADE,
        user_low_id UUID NOT NULL
            REFERENCES users(id) ON DELETE RESTRICT,
        user_high_id UUID NOT NULL
            REFERENCES users(id) ON DELETE RESTRICT,
        
        CONSTRAINT private_chat_pairs_order_check
            CHECK (user_low_id < user_high_id),

        CONSTRAINT private_chat_pairs_users_unique
            UNIQUE (user_low_id, user_high_id)
    );

-- +goose Down
DROP TABLE private_chat_pairs;
