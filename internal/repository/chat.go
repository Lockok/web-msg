package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/Lockok/web-msg/internal/repository/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *ChatRepository) GetOrCreatePrivateChat(ctx context.Context, userID uuid.UUID, peerID uuid.UUID) (*domain.Chat, error) {
	if userID == uuid.Nil || peerID == uuid.Nil {
		return nil, fmt.Errorf("%w: user IDs must not be empty", domain.ErrInvalidInput)
	}

	if userID == peerID {
		return nil, fmt.Errorf("%w: cannot create a private chat with yourself", domain.ErrInvalidInput)
	}

	lowID, highID := orderUserIDs(userID, peerID)

	chat, err := r.getPrivateChat(ctx, lowID, highID)
	if err == nil {
		return chat, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find private chat: %w", err)
	}

	chat, err = r.createPrivateChat(ctx, lowID, highID)
	if err == nil {
		return chat, nil
	}

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "private_chat_pairs_users_unique" {
		// Другой запрос создал эту пару раньше.
		// createPrivateChat уже завершил откат своей транзакции.
		chat, lookupErr := r.getPrivateChat(ctx, lowID, highID)
		if lookupErr != nil {
			return nil, fmt.Errorf(
				"find private chat after concurrent creation: %w",
				lookupErr,
			)
		}

		return chat, nil
	}

	return nil, err
}

func (r *ChatRepository) getPrivateChat(ctx context.Context, lowID uuid.UUID, highID uuid.UUID) (*domain.Chat, error) {
	chat, err := r.queries.GetPrivateChatByPair(ctx, db.GetPrivateChatByPairParams{
		UserLowID:  lowID,
		UserHighID: highID,
	},
	)
	if err != nil {
		return nil, err
	}

	return chatToDomain(chat), nil
}

func (r *ChatRepository) createPrivateChat(ctx context.Context, lowID uuid.UUID, highID uuid.UUID) (*domain.Chat, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("begin private chat transcation: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := r.queries.WithTx(tx)

	chat, err := queries.CreatePrivateChat(ctx, uuid.New())
	if err != nil {
		return nil, fmt.Errorf("create private chat: %w", err)
	}

	err = queries.CreatePrivateChatPair(ctx, db.CreatePrivateChatPairParams{
		ChatID:     chat.ID,
		UserLowID:  lowID,
		UserHighID: highID,
	})
	if err != nil {
		return nil, fmt.Errorf("create private chat pair: %w", err)
	}

	err = queries.AddPrivateChatMembers(ctx, db.AddPrivateChatMembersParams{
		ChatID:     chat.ID,
		UserLowID:  lowID,
		UserHighID: highID,
	})
	if err != nil {
		return nil, fmt.Errorf("add private chat members: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit private chat transcation: %w", err)
	}

	return chatToDomain(chat), err
}

func orderUserIDs(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(a[:], b[:]) < 0 {
		return a, b
	}

	return b, a
}

func chatToDomain(chat db.Chat) *domain.Chat {
	return &domain.Chat{
		ID:        chat.ID,
		Type:      chat.Type,
		Name:      chat.Name.String,
		CreatedAt: chat.CreatedAt.Time,
	}
}
