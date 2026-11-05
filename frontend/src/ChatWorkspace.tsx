import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import ChatRoom from './ChatRoom'

type Chat = {
  id: string
  type: string
  name?: string
}

type ChatPage = {
  items: Chat[]
  next_cursor: string | null
}

type Props = {
  username: string
  accessToken: string
  onSignOut: () => void
}

async function errorFromResponse(response: Response): Promise<Error> {
  const detail = (await response.text()).trim().slice(0, 240)
  return new Error(detail || `Ошибка сервера: ${response.status}`)
}

async function fetchChatPage(
  accessToken: string,
  cursor: string | null,
  signal?: AbortSignal,
): Promise<ChatPage> {
  const params = new URLSearchParams({ limit: '20' })

  if (cursor) {
    params.set('cursor', cursor)
  }

  const response = await fetch(`/chats?${params.toString()}`, {
    headers: { Authorization: `Bearer ${accessToken}` },
    signal,
  })

  if (!response.ok) {
    throw await errorFromResponse(response)
  }

  const data: unknown = await response.json()
  if (
    typeof data !== 'object' ||
    data === null ||
    !('items' in data) ||
    !Array.isArray(data.items) ||
    !('next_cursor' in data) ||
    (data.next_cursor !== null && typeof data.next_cursor !== 'string')
  ) {
    throw new Error('Сервер вернул некорректный список чатов')
  }

  return data as ChatPage
}

export default function ChatWorkspace({
  username,
  accessToken,
  onSignOut,
}: Props) {
  const [chats, setChats] = useState<Chat[]>([])
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [groupName, setGroupName] = useState('')
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const [selectedChat, setSelectedChat] = useState<Chat | null>(null)

  // При входе и после создания группы загружаем первую страницу.
  useEffect(() => {
    const controller = new AbortController()

    fetchChatPage(accessToken, null, controller.signal)
      .then((page) => {
        setChats(page.items)
        setNextCursor(page.next_cursor)
        setError('')
      })
      .catch((err: unknown) => {
        if (!controller.signal.aborted) {
          setError(err instanceof Error ? err.message : 'Не удалось загрузить чаты')
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false)
        }
      })

    return () => controller.abort()
  }, [accessToken, reloadKey])

  async function loadMore() {
    if (!nextCursor || loading || creating) return

    setLoading(true)
    setError('')

    try {
      const page = await fetchChatPage(accessToken, nextCursor)
      setChats((current) => [...current, ...page.items])
      setNextCursor(page.next_cursor)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить чаты')
    } finally {
      setLoading(false)
    }
  }

  async function createGroup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const name = groupName.trim()
    if (!name) {
      setError('Введите название группы')
      return
    }

    setCreating(true)
    setError('')

    try {
      const response = await fetch('/chats/group', {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${accessToken}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name }),
      })

      if (!response.ok) {
        throw await errorFromResponse(response)
      }

      setGroupName('')
      setLoading(true)
      setReloadKey((current) => current + 1)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось создать группу')
    } finally {
      setCreating(false)
    }
  }

  if (selectedChat) {
  return (
    <ChatRoom
      key={selectedChat.id}
      chat={selectedChat}
      accessToken={accessToken}
      onBack={() => setSelectedChat(null)}
    />
  )
}

  return (
    <div className="workspace">
      <div className="workspace-heading">
        <div>
          <p className="eyebrow">ВАШИ СООБЩЕНИЯ</p>
          <h1>Чаты</h1>
          <p className="description">Вы вошли как {username}</p>
        </div>
        <button className="secondary" type="button" onClick={onSignOut}>
          Выйти
        </button>
      </div>

      <form className="group-form" onSubmit={createGroup}>
        <label htmlFor="group-name">Новая группа</label>
        <input
          id="group-name"
          value={groupName}
          onChange={(event) => setGroupName(event.target.value)}
          placeholder="Название группы"
          disabled={creating}
        />
        <button className="primary" type="submit" disabled={creating || loading}>
          {creating ? 'Создаём…' : 'Создать группу'}
        </button>
      </form>

      {error && <p className="message error" role="alert">{error}</p>}

      <div className="chat-list-heading">
        <h2>Ваши чаты</h2>
        {loading && <span className="muted">Загрузка…</span>}
      </div>

      {!loading && chats.length === 0 && (
        <p className="muted">Чатов пока нет. Создайте первую группу.</p>
      )}

      <ul className="chat-list">
        {chats.map((chat) => (
          <li key={chat.id}>
  <button
    className="chat-row"
    type="button"
    onClick={() => setSelectedChat(chat)}
  >
    <span className="chat-avatar" aria-hidden="true">
      {chat.type === 'group' ? 'Г' : 'Л'}
    </span>
    <span>
      <strong>
        {chat.type === 'group' ? chat.name || 'Группа' : 'Личный чат'}
      </strong>
      <span className="chat-meta">ID: {chat.id.slice(0, 8)}</span>
    </span>
  </button>
</li>
        ))}
      </ul>

      {nextCursor && (
        <button
          className="secondary load-more"
          type="button"
          onClick={loadMore}
          disabled={loading || creating}
        >
          Загрузить ещё
        </button>
      )}

      <p className="workspace-note">
        Открытие переписки добавим на следующем шаге.
      </p>
    </div>
  )
}