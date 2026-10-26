package realtime

import (
	"context"
	"fmt"
	"time"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

const notificationTimeout = 10 * time.Second

type ChatMemberLister interface {
	ListChatMemberIDs(
		ctx context.Context,
		chatID uuid.UUID,
	) ([]uuid.UUID, error)
}

type MessageNotifier struct {
	members ChatMemberLister
	hub     *Hub
}

func NewMessageNotifier(
	members ChatMemberLister,
	hub *Hub,
) *MessageNotifier {
	return &MessageNotifier{
		members: members,
		hub:     hub,
	}
}

func (n *MessageNotifier) NotifyMessageCreated(
	ctx context.Context,
	message domain.Message,
) error {
	notifyCtx, cancel := context.WithTimeout(
		ctx,
		notificationTimeout,
	)
	defer cancel()

	userIDs, err := n.members.ListChatMemberIDs(
		notifyCtx,
		message.ChatID,
	)
	if err != nil {
		return fmt.Errorf("find message recipients: %w", err)
	}

	report := n.hub.BroadcastMessageCreated(
		notifyCtx,
		userIDs,
		message,
	)

	if err := notifyCtx.Err(); err != nil {
		return fmt.Errorf("message notification interrupted: %w", err)
	}

	if report.Failed > 0 {
		return fmt.Errorf(
			"message notification: %d of %d connection writes failed",
			report.Failed,
			report.Connections,
		)
	}

	return nil
}