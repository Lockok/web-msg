package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *ChatRepository) SetGroupMember(
	ctx context.Context,
	chatID, actorID, targetID uuid.UUID,
	add bool,
) error {
	if chatID == uuid.Nil || actorID == uuid.Nil || targetID == uuid.Nil {
		return fmt.Errorf("%w: IDs must not be empty", domain.ErrInvalidInput)
	}

	if !add && actorID == targetID {
		return fmt.Errorf("%w: admin cannot remove themselves", domain.ErrInvalidInput)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin membership transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Отправка сообщений также блокирует строку чата. Это упорядочивает
	// изменение состава относительно транзакции отправки.
	var chatType string
	err = tx.QueryRow(ctx,
		`SELECT type FROM chats WHERE id = $1 FOR UPDATE`,
		chatID,
	).Scan(&chatType)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrChatNotFound
	}
	if err != nil {
		return fmt.Errorf("lock chat: %w", err)
	}

	if chatType != "group" {
		return fmt.Errorf("%w: chat is not a group", domain.ErrInvalidInput)
	}

	var isAdmin bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM chat_members
			WHERE chat_id = $1 AND user_id = $2 AND role = 'admin'
		)
	`, chatID, actorID).Scan(&isAdmin)
	if err != nil {
		return fmt.Errorf("check group admin: %w", err)
	}
	if !isAdmin {
		return domain.ErrNotChatMember
	}

	if add {
		_, err = tx.Exec(ctx, `
			INSERT INTO chat_members (chat_id, user_id, role)
			VALUES ($1, $2, 'member')
			ON CONFLICT (chat_id, user_id) DO NOTHING
		`, chatID, targetID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) &&
				pgErr.Code == "23503" &&
				pgErr.ConstraintName == "chat_members_user_id_fkey" {
				return domain.ErrUserNotFound
			}
			return fmt.Errorf("add group member: %w", err)
		}
	} else {
		// Дополнительная защита: этим API нельзя удалить администратора.
		_, err = tx.Exec(ctx, `
			DELETE FROM chat_members
			WHERE chat_id = $1 AND user_id = $2 AND role = 'member'
		`, chatID, targetID)
		if err != nil {
			return fmt.Errorf("remove group member: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit membership transaction: %w", err)
	}
	return nil
}
