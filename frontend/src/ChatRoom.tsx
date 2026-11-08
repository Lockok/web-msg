import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'

type Chat = {
  id: string
  type: string
  name?: string
}

type Message = {
  id: string
  sender_id: string
  text: string
  seq: number
}

type MessagePage = {
  items: Message[]
  next_before_seq: number | null
}

type Props = {
  chat: Chat
  accessToken: string
  onBack: () => void
}

async function errorFromResponse(response: Response): Promise<Error> {
  const detail = (await response.text()).trim().slice(0, 240)
  return new Error(detail || `Ошибка сервера: ${response.status}`)
}

async function fetchHistory(
  chatID: string,
  accessToken: string,
  beforeSeq: number | null,
  signal?: AbortSignal,
): Promise<MessagePage> {
  const params = new URLSearchParams({ limit: '20' })

  if (beforeSeq !== null) {
    params.set('before_seq', String(beforeSeq))
  }

  const response = await fetch(
    `/chats/${chatID}/messages?${params.toString()}`,
    {
      headers: { Authorization: `Bearer ${accessToken}` },
      signal,
    },
  )

  if (!response.ok) {
    throw await errorFromResponse(response)
  }

  const data: unknown = await response.json()
  if (
    typeof data !== 'object' ||
    data === null ||
    !('items' in data) ||
    !Array.isArray(data.items) ||
    !('next_before_seq' in data) ||
    (data.next_before_seq !== null && typeof data.next_before_seq !== 'number')
  ) {
    throw new Error('Сервер вернул некорректную историю')
  }

  return data as MessagePage
}

// API возвращает историю от новых к старым.
// Для экрана упорядочиваем её от старых к новым и убираем повторы по ID.
function mergeMessages(current: Message[], incoming: Message[]): Message[] {
  const byID = new Map<string, Message>()

  for (const message of [...current, ...incoming]) {
    byID.set(message.id, message)
  }

  return [...byID.values()].sort((a, b) => a.seq - b.seq)
}

export default function ChatRoom({ chat, accessToken, onBack }: Props) {
  const [messages, setMessages] = useState<Message[]>([])
  const [nextBeforeSeq, setNextBeforeSeq] = useState<number | null>(null)
  const [text, setText] = useState('')
  const [loading, setLoading] = useState(true)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [clearing, setClearing] = useState(false)
  const [connection, setConnection] = useState<
  'connecting' | 'syncing' | 'connected' | 'disconnected' | 'stopped'
>('connecting')

const [reconnectKey, setReconnectKey] = useState(0)

  const sendingRef = useRef(false)
  const pendingRef = useRef<{ text: string; clientID: string } | null>(null)
  const syncedSeqRef = useRef(0)
  const clearingRef = useRef(false)
  const hiddenThroughRef = useRef(0)
  const historyGenerationRef = useRef(0)

  useEffect(() => {
    const controller = new AbortController()

    fetchHistory(chat.id, accessToken, null, controller.signal)
      .then((page) => {
  for (const message of page.items) {
    if (!Number.isSafeInteger(message.seq) || message.seq <= 0) {
      throw new Error('Некорректный номер сообщения в истории')
    }
  }

  syncedSeqRef.current = page.items.reduce(
    (max, message) => Math.max(max, message.seq),
    0,
  )

  setMessages(mergeMessages([], page.items))
  setNextBeforeSeq(page.next_before_seq)
})
      .catch((err: unknown) => {
        if (!controller.signal.aborted) {
          setError(err instanceof Error ? err.message : 'Не удалось загрузить историю')
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false)
        }
      })

    return () => controller.abort()
  }, [chat.id, accessToken])

useEffect(() => {
  if (loading) return

  let active = true
  let attempts = 0
  let retryTimer: number | undefined
  let currentSocket: WebSocket | undefined
  let currentSync: AbortController | undefined

  async function syncMessages(
    controller: AbortController,
    stopRetry: () => void,
  ) {
    while (active && !controller.signal.aborted) {
      const afterSeq = syncedSeqRef.current

      const params = new URLSearchParams({
        after_seq: String(afterSeq),
        limit: '100',
      })

      const response = await fetch(
        `/chats/${chat.id}/messages/sync?${params.toString()}`,
        {
          headers: { Authorization: `Bearer ${accessToken}` },
          signal: controller.signal,
        },
      )

      if (!response.ok) {
        // Ошибку доступа бессмысленно повторять автоматически.
        if (response.status >= 400 && response.status < 500) {
          stopRetry()
        }
        throw await errorFromResponse(response)
      }

      const data: unknown = await response.json()

      if (
        typeof data !== 'object' ||
        data === null ||
        !('items' in data) ||
        !Array.isArray(data.items) ||
        !('next_after_seq' in data) ||
        (data.next_after_seq !== null &&
          typeof data.next_after_seq !== 'number')
      ) {
        throw new Error('Сервер вернул некорректную страницу восстановления')
      }

      const rows = data.items as Message[]
      let lastSeq = afterSeq

      for (const message of rows) {
        if (
          typeof message.id !== 'string' ||
          typeof message.sender_id !== 'string' ||
          typeof message.text !== 'string' ||
          !Number.isSafeInteger(message.seq) ||
          message.seq <= lastSeq
        ) {
          throw new Error('Некорректное сообщение при восстановлении')
        }
        lastSeq = message.seq
      }

      if (
        data.next_after_seq !== null &&
        (rows.length === 0 || data.next_after_seq !== lastSeq)
      ) {
        throw new Error('Некорректный курсор восстановления')
      }

      if (!active || controller.signal.aborted) return

      setMessages((current) =>
  mergeMessages(current, rows).filter(
    (message) => message.seq > hiddenThroughRef.current,
  ),
)
syncedSeqRef.current = Math.max(syncedSeqRef.current, lastSeq)

      if (data.next_after_seq === null) return
    }
  }

  function connect() {
    if (!active) return

    let stopAutomaticRetry = false
    const controller = new AbortController()
    currentSync = controller

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const socket = new WebSocket(`${protocol}//${window.location.host}/ws`)
    currentSocket = socket

    socket.onopen = () => {
      if (!active) return

      socket.send(JSON.stringify({
        type: 'auth',
        access_token: accessToken,
      }))
    }

    socket.onmessage = (event) => {
      if (!active || typeof event.data !== 'string') return

      let data: unknown
      try {
        data = JSON.parse(event.data)
      } catch {
        return
      }

      if (typeof data !== 'object' || data === null || !('type' in data)) {
        return
      }

      if (data.type === 'auth.ok') {
        setConnection('syncing')

        syncMessages(controller, () => {
          stopAutomaticRetry = true
        })
          .then(() => {
            if (active && !controller.signal.aborted) {
              attempts = 0
              setError('')
              setConnection('connected')
            }
          })
          .catch((err: unknown) => {
            if (!active || controller.signal.aborted) return

            setError(err instanceof Error ? err.message : 'Ошибка восстановления')
            socket.close()
          })

        return
      }

      if (
        data.type !== 'message.created' ||
        !('message' in data) ||
        typeof data.message !== 'object' ||
        data.message === null
      ) {
        return
      }

      const message = data.message
      if (
        !('chat_id' in message) ||
        message.chat_id !== chat.id ||
        !('id' in message) ||
        typeof message.id !== 'string' ||
        !('sender_id' in message) ||
        typeof message.sender_id !== 'string' ||
        !('text' in message) ||
        typeof message.text !== 'string' ||
        !('seq' in message) ||
        !Number.isSafeInteger(message.seq) ||
        (message.seq as number) <= 0
      ) {
        return
      }

      // События можно показывать сразу, но sync-курсор по ним не двигаем.
      setMessages((current) =>
  mergeMessages(current, [{
    id: message.id as string,
    sender_id: message.sender_id as string,
    text: message.text as string,
    seq: message.seq as number,
  }]).filter((item) => item.seq > hiddenThroughRef.current),
)
    }

    socket.onerror = () => {
      if (active) setConnection('disconnected')
    }

    socket.onclose = (event) => {
      controller.abort()
      if (!active) return

      if (event.code === 1008) {
        stopAutomaticRetry = true
        setError('Соединение отклонено. Возможно, нужно войти снова.')
      }

      attempts++

      if (stopAutomaticRetry || attempts >= 5) {
        setConnection('stopped')
        return
      }

      setConnection('disconnected')

      retryTimer = window.setTimeout(() => {
        if (!active) return
        setConnection('connecting')
        connect()
      }, Math.min(1000 * 2 ** (attempts - 1), 8000))
    }
  }

  connect()

  return () => {
    active = false
    currentSync?.abort()

    if (retryTimer !== undefined) {
      window.clearTimeout(retryTimer)
    }

    currentSocket?.close()
  }
}, [accessToken, chat.id, loading, reconnectKey])

  async function loadOlder() {
    if (nextBeforeSeq === null || loadingOlder) return

    const generation = historyGenerationRef.current

    setLoadingOlder(true)
    setError('')

    try {
      const page = await fetchHistory(chat.id, accessToken, nextBeforeSeq)
      if (generation !== historyGenerationRef.current) return

setMessages((current) =>
  mergeMessages(current, page.items).filter(
    (message) => message.seq > hiddenThroughRef.current,
  ),
)
setNextBeforeSeq(page.next_before_seq)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось загрузить историю')
    } finally {
      setLoadingOlder(false)
    }
  }

  async function sendMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (loading || sendingRef.current || text.trim() === '') return

    sendingRef.current = true
    setSending(true)
    setError('')

    try {
      // При повторной отправке того же текста после ошибки используем
      // прежний ключ: сервер сможет распознать уже сохранённое сообщение.
      if (pendingRef.current === null || pendingRef.current.text !== text) {
        pendingRef.current = {
          text,
          clientID: crypto.randomUUID(),
        }
      }

      const pending = pendingRef.current
      const response = await fetch(`/chats/${chat.id}/messages`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${accessToken}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          client_message_id: pending.clientID,
          text: pending.text,
        }),
      })

      if (!response.ok) {
        throw await errorFromResponse(response)
      }

      const message = (await response.json()) as Message
      setMessages((current) =>
  mergeMessages(current, [message]).filter(
    (item) => item.seq > hiddenThroughRef.current,
  ),
)
      setText('')
      pendingRef.current = null
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось отправить сообщение')
    } finally {
      sendingRef.current = false
      setSending(false)
    }
  }

  async function clearHistory() {
  if (loading || clearingRef.current) return

  const confirmed = window.confirm(
    'Очистить историю только для себя? У других участников сообщения останутся. Сообщения не удаляются из базы.',
  )
  if (!confirmed) return

  clearingRef.current = true
  setClearing(true)
  setError('')

  try {
    const response = await fetch(`/chats/${chat.id}/history/clear`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${accessToken}` },
    })

    if (!response.ok) {
      throw await errorFromResponse(response)
    }

    const data: unknown = await response.json()
    if (
      typeof data !== 'object' ||
      data === null ||
      !('cleared_through_seq' in data) ||
      typeof data.cleared_through_seq !== 'number' ||
      !Number.isSafeInteger(data.cleared_through_seq) ||
      data.cleared_through_seq < 0
    ) {
      throw new Error('Некорректный ответ сервера. Переоткройте чат и проверьте историю.')
    }

    const cutoff = data.cleared_through_seq
    hiddenThroughRef.current = Math.max(hiddenThroughRef.current, cutoff)
    syncedSeqRef.current = Math.max(syncedSeqRef.current, cutoff)
    historyGenerationRef.current++

    setMessages((current) =>
      current.filter((message) => message.seq > hiddenThroughRef.current),
    )
    setNextBeforeSeq(null)
  } catch (err) {
    const detail = err instanceof Error ? err.message : 'Неизвестная ошибка'
    setError(
      `${detail}. Если ответ потерялся, переоткройте чат и проверьте историю перед повторной очисткой.`,
    )
  } finally {
    clearingRef.current = false
    setClearing(false)
  }
}

  const title = chat.type === 'group' ? chat.name || 'Группа' : 'Личный чат'

  return (
    <section className="room">
      <div className="room-heading">
        <div>
          <button className="back-button" type="button" onClick={onBack}>
            ← Все чаты
          </button>
          <h1>{title}</h1>
          <p className="muted">ID: {chat.id.slice(0, 8)}</p>
        </div>
      </div>

      <button
  className="secondary"
  type="button"
  onClick={clearHistory}
  disabled={loading || clearing}
>
  {clearing ? 'Очищаем…' : 'Очистить историю для себя'}
</button>

      {error && <p className="message error" role="alert">{error}</p>}

      {loading && <p className="muted">Загружаем историю…</p>}

      {nextBeforeSeq !== null && (
        <button
          className="secondary load-more"
          type="button"
          onClick={loadOlder}
          disabled={loadingOlder}
        >
          {loadingOlder ? 'Загрузка…' : 'Загрузить более ранние'}
        </button>
      )}

      {!loading && messages.length === 0 && (
        <p className="muted">Сообщений пока нет. Напишите первое.</p>
      )}

      <ul className="message-list">
        {messages.map((message) => (
          <li className="message-item" key={message.id}>
            <span className="message-author">
              Участник {message.sender_id.slice(0, 8)}
            </span>
            <p>{message.text}</p>
          </li>
        ))}
      </ul>

      <form className="message-form" onSubmit={sendMessage}>
        <label htmlFor="message-text">Сообщение</label>
        <textarea
          id="message-text"
          value={text}
          onChange={(event) => setText(event.target.value)}
          placeholder="Напишите сообщение"
          rows={3}
          disabled={sending}
        />
        <button
          className="primary"
          type="submit"
          disabled={loading || sending || text.trim() === ''}
        >
          {sending ? 'Отправляем…' : 'Отправить'}
        </button>
      </form>

    <p className="workspace-note" aria-live="polite">
  {connection === 'connecting' && 'Подключаемся к обновлениям…'}
  {connection === 'syncing' && 'Восстанавливаем сообщения…'}
  {connection === 'connected' && 'Обновления подключены'}
  {connection === 'disconnected' && 'Связь потеряна. Пробуем подключиться снова…'}
  {connection === 'stopped' && 'Не удалось восстановить соединение.'}
</p>

{connection === 'stopped' && (
  <button
    className="secondary"
    type="button"
    onClick={() => {
      setError('')
      setConnection('connecting')
      setReconnectKey((current) => current + 1)
    }}
  >
    Подключиться снова
  </button>
)}
    </section>
  )
}