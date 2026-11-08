//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Lockok/web-msg/internal/realtime"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

func TestGroupChatE2E(t *testing.T) {
	baseURL := os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	request := func(
		method, path, token string,
		body any,
		wantStatus int,
		result any,
	) {
		t.Helper()

		var reader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("encode %s %s: %v", method, path, err)
			}
			reader = bytes.NewReader(data)
		}

		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
		if err != nil {
			t.Fatalf("create %s %s: %v", method, path, err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			t.Fatalf("read %s %s response: %v", method, path, err)
		}
		if resp.StatusCode != wantStatus {
			t.Fatalf(
				"%s %s: expected %d, got %d: %s",
				method, path, wantStatus, resp.StatusCode, data,
			)
		}
		if result != nil {
			if err := json.Unmarshal(data, result); err != nil {
				t.Fatalf("decode %s %s response: %v; body=%s", method, path, err, data)
			}
		}
	}

	createAccount := func() (uuid.UUID, string) {
		t.Helper()

		credentials := map[string]string{
			"username": "e2e_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12],
			"password": "LocalTest_12345!",
		}

		var user struct {
			ID uuid.UUID `json:"id"`
		}
		request(http.MethodPost, "/register", "", credentials, http.StatusCreated, &user)

		var login struct {
			AccessToken string `json:"access_token"`
		}
		request(http.MethodPost, "/login", "", credentials, http.StatusOK, &login)

		if user.ID == uuid.Nil || login.AccessToken == "" {
			t.Fatal("registration or login returned incomplete data")
		}
		return user.ID, login.AccessToken
	}

	adminID, adminToken := createAccount()
	memberID, memberToken := createAccount()
	outsiderID, outsiderToken := createAccount()

	// Создание требует авторизации; название нормализуется сервисом.
	request(
		http.MethodPost, "/chats/group", "",
		map[string]string{"name": "Команда"},
		http.StatusUnauthorized, nil,
	)

	var group struct {
		ID   uuid.UUID `json:"id"`
		Type string    `json:"type"`
		Name string    `json:"name"`
	}
	request(
		http.MethodPost, "/chats/group", adminToken,
		map[string]string{"name": "  Команда  "},
		http.StatusCreated, &group,
	)
	if group.ID == uuid.Nil || group.Type != "group" || group.Name != "Команда" {
		t.Fatalf("unexpected group response: %+v", group)
	}

	memberPath := fmt.Sprintf("/chats/%s/members/%s", group.ID, memberID)
	outsiderPath := fmt.Sprintf("/chats/%s/members/%s", group.ID, outsiderID)
	adminPath := fmt.Sprintf("/chats/%s/members/%s", group.ID, adminID)
	messagesPath := fmt.Sprintf("/chats/%s/messages", group.ID)
	syncPath := fmt.Sprintf("/chats/%s/messages/sync?after_seq=0&limit=20", group.ID)

	// Посторонний не может добавить себя или читать группу.
	request(http.MethodPut, outsiderPath, outsiderToken, nil, http.StatusForbidden, nil)
	request(http.MethodGet, syncPath, outsiderToken, nil, http.StatusForbidden, nil)

	// Администратор добавляет участника; повторный запрос безопасен.
	request(http.MethodPut, memberPath, adminToken, nil, http.StatusNoContent, nil)
	request(http.MethodPut, memberPath, adminToken, nil, http.StatusNoContent, nil)

	// Обычный участник не может добавлять других.
	request(http.MethodPut, outsiderPath, memberToken, nil, http.StatusForbidden, nil)

	// Администратор не может удалить себя.
	request(http.MethodDelete, adminPath, adminToken, nil, http.StatusBadRequest, nil)

	wsURL := strings.Replace(baseURL, "http", "ws", 1) + "/ws"

	memberConn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("connect member WebSocket: %v", err)
	}
	t.Cleanup(func() {
		_ = memberConn.CloseNow()
	})

	err = wsjson.Write(ctx, memberConn, map[string]string{
		"type":         "auth",
		"access_token": memberToken,
	})
	if err != nil {
		t.Fatalf("authenticate member WebSocket: %v", err)
	}

	var authResponse struct {
		Type string `json:"type"`
	}
	if err := wsjson.Read(ctx, memberConn, &authResponse); err != nil {
		t.Fatalf("read WebSocket auth response: %v", err)
	}
	if authResponse.Type != "auth.ok" {
		t.Fatalf("expected auth.ok, got %q", authResponse.Type)
	}

	// Участник отправляет сообщение.
	var sent struct {
		ID       uuid.UUID `json:"id"`
		ChatID   uuid.UUID `json:"chat_id"`
		SenderID uuid.UUID `json:"sender_id"`
		Text     string    `json:"text"`
		Seq      int64     `json:"seq"`
	}
	request(
		http.MethodPost, messagesPath, memberToken,
		map[string]string{
			"client_message_id": uuid.NewString(),
			"text":              "Привет группе",
		},
		http.StatusOK, &sent,
	)
	if sent.ID == uuid.Nil || sent.ChatID != group.ID ||
		sent.SenderID != memberID || sent.Text != "Привет группе" || sent.Seq != 1 {
		t.Fatalf("unexpected sent message: %+v", sent)
	}

	var event realtime.MessageCreatedEvent
	if err := wsjson.Read(ctx, memberConn, &event); err != nil {
		t.Fatalf("read group message event: %v", err)
	}
	if event.Type != "message.created" ||
		event.Message.ID != sent.ID ||
		event.Message.ChatID != group.ID ||
		event.Message.SenderID != memberID ||
		event.Message.Text != sent.Text ||
		event.Message.Seq != sent.Seq {
		t.Fatalf("unexpected group event: %+v", event)
	}

	// Администратор видит сообщение в восстановлении.
	var page struct {
		Items []struct {
			ID       uuid.UUID `json:"id"`
			SenderID uuid.UUID `json:"sender_id"`
			Text     string    `json:"text"`
			Seq      int64     `json:"seq"`
		} `json:"items"`
	}
	request(http.MethodGet, syncPath, adminToken, nil, http.StatusOK, &page)
	if len(page.Items) != 1 ||
		page.Items[0].ID != sent.ID ||
		page.Items[0].SenderID != memberID ||
		page.Items[0].Text != sent.Text ||
		page.Items[0].Seq != sent.Seq {
		t.Fatalf("unexpected sync page: %+v", page)
	}

	// После удаления бывший участник не может читать или отправлять.
	request(http.MethodDelete, memberPath, adminToken, nil, http.StatusNoContent, nil)
	request(http.MethodDelete, memberPath, adminToken, nil, http.StatusNoContent, nil)
	request(http.MethodGet, syncPath, memberToken, nil, http.StatusForbidden, nil)
	request(
		http.MethodPost, messagesPath, memberToken,
		map[string]string{
			"client_message_id": uuid.NewString(),
			"text":              "После удаления",
		},
		http.StatusForbidden, nil,
	)

	// Удаление участника не должно лишить администратора доступа.
	request(http.MethodGet, syncPath, adminToken, nil, http.StatusOK, nil)

	var afterRemoval struct {
		ID  uuid.UUID `json:"id"`
		Seq int64     `json:"seq"`
	}

	request(
		http.MethodPost, messagesPath, adminToken,
		map[string]string{
			"client_message_id": uuid.NewString(),
			"text":              "Сообщение после удаления",
		},
		http.StatusOK, &afterRemoval,
	)
	if afterRemoval.ID == uuid.Nil || afterRemoval.Seq != 2 {
		t.Fatalf("unexpected message after removal: %+v", afterRemoval)
	}

	// Соединение бывшего участника всё ещё открыто,
	// но новое событие группы ему приходить не должно.
	readCtx, cancelRead := context.WithTimeout(ctx, time.Second)
	defer cancelRead()

	_, _, err = memberConn.Read(readCtx)
	if err == nil {
		t.Fatal("removed member received an unexpected event")
	}
	if readCtx.Err() != context.DeadlineExceeded {
		t.Fatalf("member connection failed unexpectedly: %v", err)
	}

	// Подготовим два сообщения для поиска.
	var searchMessage1, searchMessage2 struct {
		ID  uuid.UUID `json:"id"`
		Seq int64     `json:"seq"`
	}

	request(
		http.MethodPost, messagesPath, adminToken,
		map[string]string{
			"client_message_id": uuid.NewString(),
			"text":              "hello %_ one",
		},
		http.StatusOK, &searchMessage1,
	)
	request(
		http.MethodPost, messagesPath, adminToken,
		map[string]string{
			"client_message_id": uuid.NewString(),
			"text":              "HELLO second",
		},
		http.StatusOK, &searchMessage2,
	)

	if searchMessage1.ID == uuid.Nil || searchMessage1.Seq != 3 ||
		searchMessage2.ID == uuid.Nil || searchMessage2.Seq != 4 {
		t.Fatalf("unexpected search messages: %+v %+v", searchMessage1, searchMessage2)
	}

	searchPath := fmt.Sprintf("/chats/%s/messages/search", group.ID)

	type searchPage struct {
		Items []struct {
			ID  uuid.UUID `json:"id"`
			Seq int64     `json:"seq"`
		} `json:"items"`
		NextBeforeSeq *int64 `json:"next_before_seq"`
	}

	// Первая страница: регистр не важен, сначала более новое сообщение.
	var firstSearchPage searchPage
	request(
		http.MethodGet, searchPath+"?q=hello&limit=1", adminToken,
		nil, http.StatusOK, &firstSearchPage,
	)
	if len(firstSearchPage.Items) != 1 ||
		firstSearchPage.Items[0].ID != searchMessage2.ID ||
		firstSearchPage.NextBeforeSeq == nil ||
		*firstSearchPage.NextBeforeSeq != searchMessage2.Seq {
		t.Fatalf("unexpected first search page: %+v", firstSearchPage)
	}

	// Продолжение начинается строго до курсора.
	var secondSearchPage searchPage
	request(
		http.MethodGet,
		fmt.Sprintf("%s?q=hello&limit=1&before_seq=%d",
			searchPath, *firstSearchPage.NextBeforeSeq),
		adminToken, nil, http.StatusOK, &secondSearchPage,
	)
	if len(secondSearchPage.Items) != 1 ||
		secondSearchPage.Items[0].ID != searchMessage1.ID ||
		secondSearchPage.NextBeforeSeq != nil {
		t.Fatalf("unexpected second search page: %+v", secondSearchPage)
	}

	// %25 — URL-кодирование символа %. Поиск должен найти именно "%_".
	var literalSearchPage searchPage
	request(
		http.MethodGet, searchPath+"?q=%25_", adminToken,
		nil, http.StatusOK, &literalSearchPage,
	)
	if len(literalSearchPage.Items) != 1 ||
		literalSearchPage.Items[0].ID != searchMessage1.ID {
		t.Fatalf("unexpected literal search page: %+v", literalSearchPage)
	}

	// Пустой результат — массив [], а не null.
	var emptySearchPage searchPage
	request(
		http.MethodGet, searchPath+"?q=no-such-fragment", adminToken,
		nil, http.StatusOK, &emptySearchPage,
	)
	if emptySearchPage.Items == nil || len(emptySearchPage.Items) != 0 {
		t.Fatalf("expected empty search array: %+v", emptySearchPage)
	}

	// Доступ и валидация на настоящем маршруте.
	request(http.MethodGet, searchPath+"?q=hello", "", nil, http.StatusUnauthorized, nil)
	request(http.MethodGet, searchPath+"?q=hello", outsiderToken, nil, http.StatusForbidden, nil)
	request(http.MethodGet, searchPath+"?q=hello", memberToken, nil, http.StatusForbidden, nil)
	request(http.MethodGet, searchPath+"?q=", adminToken, nil, http.StatusBadRequest, nil)
	request(http.MethodGet, searchPath+"?q=a&q=b", adminToken, nil, http.StatusBadRequest, nil)

	// Вернём ранее удалённого участника в группу.
	request(http.MethodPut, memberPath, adminToken, nil, http.StatusNoContent, nil)

	clearPath := fmt.Sprintf("/chats/%s/history/clear", group.ID)

	// Без токена и без участия в чате очистка запрещена.
	request(http.MethodPost, clearPath, "", nil, http.StatusUnauthorized, nil)
	request(http.MethodPost, clearPath, outsiderToken, nil, http.StatusForbidden, nil)

	var clearance struct {
		ClearedThroughSeq int64 `json:"cleared_through_seq"`
	}
	request(http.MethodPost, clearPath, memberToken, nil, http.StatusOK, &clearance)

	if clearance.ClearedThroughSeq != searchMessage2.Seq {
		t.Fatalf("unexpected clearance: %+v", clearance)
	}

	// У участника очищенная история не видна ни в истории, ни в поиске, ни в sync.
	var memberHistory searchPage
	request(http.MethodGet, messagesPath+"?limit=20", memberToken, nil, http.StatusOK, &memberHistory)
	if len(memberHistory.Items) != 0 {
		t.Fatalf("cleared history is still visible: %+v", memberHistory)
	}

	var memberSearch searchPage
	request(http.MethodGet, searchPath+"?q=hello", memberToken, nil, http.StatusOK, &memberSearch)
	if len(memberSearch.Items) != 0 {
		t.Fatalf("cleared messages are still searchable: %+v", memberSearch)
	}

	var memberSync struct {
		Items []struct {
			ID  uuid.UUID `json:"id"`
			Seq int64     `json:"seq"`
		} `json:"items"`
	}
	request(http.MethodGet, syncPath, memberToken, nil, http.StatusOK, &memberSync)
	if len(memberSync.Items) != 0 {
		t.Fatalf("cleared messages are still in sync: %+v", memberSync)
	}

	// Администратор по-прежнему видит старые сообщения.
	var adminSearch searchPage
	request(http.MethodGet, searchPath+"?q=hello", adminToken, nil, http.StatusOK, &adminSearch)
	if len(adminSearch.Items) != 2 {
		t.Fatalf("other participant lost search results: %+v", adminSearch)
	}

	var adminSync struct {
		Items []struct {
			ID  uuid.UUID `json:"id"`
			Seq int64     `json:"seq"`
		} `json:"items"`
	}
	request(http.MethodGet, syncPath, adminToken, nil, http.StatusOK, &adminSync)
	if len(adminSync.Items) != int(searchMessage2.Seq) {
		t.Fatalf("other participant lost history: %+v", adminSync)
	}

	// Новое сообщение после очистки видно обоим.
	var afterClear struct {
		ID  uuid.UUID `json:"id"`
		Seq int64     `json:"seq"`
	}
	request(
		http.MethodPost, messagesPath, adminToken,
		map[string]string{
			"client_message_id": uuid.NewString(),
			"text":              "Сообщение после очистки",
		},
		http.StatusOK, &afterClear,
	)
	if afterClear.ID == uuid.Nil || afterClear.Seq != clearance.ClearedThroughSeq+1 {
		t.Fatalf("unexpected new message: %+v", afterClear)
	}

	var memberAfterClear searchPage
	request(http.MethodGet, messagesPath+"?limit=20", memberToken, nil, http.StatusOK, &memberAfterClear)
	if len(memberAfterClear.Items) != 1 || memberAfterClear.Items[0].ID != afterClear.ID {
		t.Fatalf("unexpected history after clearance: %+v", memberAfterClear)
	}
}
