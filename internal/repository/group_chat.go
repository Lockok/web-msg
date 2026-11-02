package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/repository/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *ChatRepository) CreateGroupChat(
	ctx context.Context,
	input domain.CreateGroupChatInput,
) (*domain.Chat, error) {
	if input.CreatorID == uuid.Nil || strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf(
			"%w: creator ID and group name are required",
			domain.ErrInvalidInput,
		)
	}

	tx, err := r.pool.BeginTx(
		ctx,
		pgx.TxOptions{IsoLevel: pgx.ReadCommitted},
	)
	if err != nil {
		return nil, fmt.Errorf("begin group creation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := r.queries.WithTx(tx)

	chat, err := queries.CreateGroupChat(
		ctx,
		db.CreateGroupChatParams{
			ID:   uuid.New(),
			Name: input.Name,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create group chat: %w", err)
	}

	err = queries.AddGroupChatCreator(
		ctx,
		db.AddGroupChatCreatorParams{
			ChatID: chat.ID,
			UserID: input.CreatorID,
		},
	)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23503" &&
			pgErr.ConstraintName == "chat_members_user_id_fkey" {
			return nil, fmt.Errorf(
				"add group creator: %w",
				domain.ErrUserNotFound,
			)
		}

		return nil, fmt.Errorf("add group creator: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit group creation transaction: %w", err)
	}

	return chatToDomain(chat), nil
}
