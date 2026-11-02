package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

// Сохраняет совместимость существующих тестов с расширенным интерфейсом.
func (m *mockMessageRepository) SearchChatMessages(
	ctx context.Context,
	chatID, userID uuid.UUID,
	searchText string,
	pageSize int32,
	beforeSeq *int64,
) ([]domain.Message, error) {
	return nil, errors.New("unexpected search call")
}

type searchRepositoryStub struct {
	*mockMessageRepository
	rows       []domain.Message
	err        error
	calls      int
	searchText string
	pageSize   int32
}

func (m *searchRepositoryStub) SearchChatMessages(
	ctx context.Context,
	chatID, userID uuid.UUID,
	searchText string,
	pageSize int32,
	beforeSeq *int64,
) ([]domain.Message, error) {
	m.calls++
	m.searchText = searchText
	m.pageSize = pageSize
	return m.rows, m.err
}

func TestMessageService_SearchChatMessages(t *testing.T) {
	repo := &searchRepositoryStub{
		rows: []domain.Message{{Seq: 5}, {Seq: 4}, {Seq: 3}},
	}
	svc := NewMessageService(repo)

	page, err := svc.SearchChatMessages(
		context.Background(), uuid.New(), uuid.New(), "  hello  ", 2, nil,
	)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if repo.calls != 1 || repo.searchText != "hello" || repo.pageSize != 3 {
		t.Fatalf("unexpected repository call: %+v", repo)
	}
	if len(page.Items) != 2 || page.Items[0].Seq != 5 ||
		page.Items[1].Seq != 4 ||
		page.NextBeforeSeq == nil || *page.NextBeforeSeq != 4 {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestMessageService_SearchChatMessagesValidation(t *testing.T) {
	zero := int64(0)
	tests := []struct {
		name      string
		query     string
		limit     int
		beforeSeq *int64
	}{
		{"empty", " ", 20, nil},
		{"oversized", strings.Repeat("a", 257), 20, nil},
		{"invalid UTF-8", string([]byte{0xff}), 20, nil},
		{"NUL", "a\x00b", 20, nil},
		{"zero limit", "hello", 0, nil},
		{"oversized limit", "hello", 101, nil},
		{"zero cursor", "hello", 20, &zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &searchRepositoryStub{}
			svc := NewMessageService(repo)

			page, err := svc.SearchChatMessages(
				context.Background(), uuid.New(), uuid.New(),
				tt.query, tt.limit, tt.beforeSeq,
			)
			if !errors.Is(err, domain.ErrInvalidInput) || page != nil || repo.calls != 0 {
				t.Fatalf("expected validation error without repository call: page=%v err=%v", page, err)
			}
		})
	}
}

func TestMessageService_SearchChatMessagesEmptyAndError(t *testing.T) {
	repo := &searchRepositoryStub{}
	svc := NewMessageService(repo)

	page, err := svc.SearchChatMessages(
		context.Background(), uuid.New(), uuid.New(), "hello", 20, nil,
	)
	if err != nil || page == nil || page.Items == nil || page.NextBeforeSeq != nil {
		t.Fatalf("expected empty page: page=%v err=%v", page, err)
	}

	repo.err = domain.ErrNotChatMember
	page, err = svc.SearchChatMessages(
		context.Background(), uuid.New(), uuid.New(), "hello", 20, nil,
	)
	if page != nil || !errors.Is(err, domain.ErrNotChatMember) {
		t.Fatalf("expected wrapped access error: page=%v err=%v", page, err)
	}
}
