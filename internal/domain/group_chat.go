package domain

import "github.com/google/uuid"

type CreateGroupChatInput struct {
	CreatorID uuid.UUID
	Name      string
}
