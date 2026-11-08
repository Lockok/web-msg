package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *ChatRepository) ClearChatHistory(
	ctx context.Context,
	chatID, userID uuid.UUID,
) (int64, error) {
	if chatID == uuid.Nil || userID == uuid.Nil {
		return 0, fmt.Errorf("%w: IDs must not be empty", domain.ErrInvalidInput)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, fmt.Errorf("begin history clearance: %w", err)
	}
	defer tx.Rollback(ctx)

	// Отправка сообщения и изменение участников используют ту же
	// блокировку строки чата. Это определяет границу при конкуренции.
	var lastSeq int64
	err = tx.QueryRow(ctx, `
		SELECT last_message_seq FROM chats WHERE id = $1 FOR UPDATE
	`, chatID).Scan(&lastSeq)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrChatNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lock chat for clearance: %w", err)
	}

	var isMember bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM chat_members
			WHERE chat_id = $1 AND user_id = $2
		)
	`, chatID, userID).Scan(&isMember)
	if err != nil {
		return 0, fmt.Errorf("check clearance membership: %w", err)
	}
	if !isMember {
		return 0, domain.ErrNotChatMember
	}

	var clearedThrough int64
	err = tx.QueryRow(ctx, `
		INSERT INTO chat_history_clearances
		    (chat_id, user_id, cleared_through_seq)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, user_id)
		DO UPDATE SET cleared_through_seq = GREATEST(
			chat_history_clearances.cleared_through_seq,
			EXCLUDED.cleared_through_seq
		)
		RETURNING cleared_through_seq
	`, chatID, userID, lastSeq).Scan(&clearedThrough)
	if err != nil {
		return 0, fmt.Errorf("save history clearance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit history clearance: %w", err)
	}

	return clearedThrough, nil
}
