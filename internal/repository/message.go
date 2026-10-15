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
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *MessageRepository) Send(ctx context.Context, input domain.SendMessageInput) (*domain.Message, error) {
	if input.ChatID == uuid.Nil ||
		input.SenderID == uuid.Nil ||
		input.ClientMessageID == uuid.Nil {
		return nil, fmt.Errorf(
			"%w: message IDs must not be empty",
			domain.ErrInvalidInput,
		)
	}

	if strings.TrimSpace(input.Text) == "" {
		return nil, fmt.Errorf("%w: message text must not be blank", domain.ErrInvalidInput)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("begin send message transcation: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := r.queries.WithTx(tx)

	_, err = queries.LockChatForMessage(ctx, input.ChatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}

		return nil, fmt.Errorf("lock chat for message: %w", err)
	}

	isMember, err := queries.IsChatMember(
		ctx,
		db.IsChatMemberParams{
			ChatID: input.ChatID,
			UserID: input.SenderID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("check message sender membership: %w", err)
	}

	if !isMember {
		return nil, domain.ErrNotChatMember
	}

	existing, err := queries.GetMessageByClientID(
		ctx,
		db.GetMessageByClientIDParams{
			ChatID:          input.ChatID,
			SenderID:        input.SenderID,
			ClientMessageID: input.ClientMessageID,
		},
	)

	if err == nil {
		if existing.Text != input.Text {
			return nil, domain.ErrMessageConflict
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit repeated message transaction: %w", err)
		}

		return messageToDomain(existing), nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find repeated message: %w", err)
	}

	seq, err := queries.IncrementChatMessageSeq(ctx, input.ChatID)
	if err != nil {
		return nil, fmt.Errorf("increment message sequence: %w", err)
	}

	message, err := queries.CreateMessage(
		ctx,
		db.CreateMessageParams{
			ID:              uuid.New(),
			ChatID:          input.ChatID,
			SenderID:        input.SenderID,
			Text:            input.Text,
			Seq:             seq,
			ClientMessageID: input.ClientMessageID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit send message transaction: %w", err)
	}

	return messageToDomain(message), nil

}

func messageToDomain(message db.Message) *domain.Message {
	return &domain.Message{
		ID:              message.ID,
		ChatID:          message.ChatID,
		SenderID:        message.SenderID,
		Text:            message.Text,
		Seq:             message.Seq,
		ClientMessageID: message.ClientMessageID,
		CreatedAt:       message.CreatedAt.Time,
	}
}
