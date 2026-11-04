import { useState } from 'react'
import type { FormEvent } from 'react'
import ChatWorkspace from './ChatWorkspace'

type Mode = 'login' | 'register'

type Session = {
  username: string
  accessToken: string
}

async function submitCredentials(
  mode: Mode,
  username: string,
  password: string,
): Promise<string | null> {
  const response = await fetch(mode === 'login' ? '/login' : '/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })

  if (!response.ok) {
    const detail = (await response.text()).trim().slice(0, 240)
    throw new Error(detail || `Ошибка сервера: ${response.status}`)
  }

  if (mode === 'register') {
    return null
  }

  const data: unknown = await response.json()

  if (
    typeof data !== 'object' ||
    data === null ||
    !('access_token' in data) ||
    typeof data.access_token !== 'string' ||
    data.access_token === ''
  ) {
    throw new Error('Сервер вернул ответ без токена')
  }

  return data.access_token
}

export default function App() {
  const [mode, setMode] = useState<Mode>('login')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [session, setSession] = useState<Session | null>(null)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    setNotice('')
    setBusy(true)

    try {
      const token = await submitCredentials(mode, username, password)
      setPassword('')

      if (token === null) {
        setMode('login')
        setNotice('Аккаунт создан. Теперь войдите.')
        return
      }

      setSession({ username, accessToken: token })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Не удалось связаться с сервером')
    } finally {
      setBusy(false)
    }
  }

  function changeMode(next: Mode) {
    setMode(next)
    setError('')
    setNotice('')
    setPassword('')
  }

  return (
    <main className="page">
      <section className="card">
        <div className="brand">
          <span className="brand-icon" aria-hidden="true">✦</span>
          <span>web-msg</span>
        </div>

        {session ? (
  <ChatWorkspace
    username={session.username}
    accessToken={session.accessToken}
    onSignOut={() => {
      setSession(null)
      setUsername('')
    }}
  />
) : (
          <>
            <p className="eyebrow">ВАШИ СООБЩЕНИЯ В ОДНОМ МЕСТЕ</p>
            <h1>{mode === 'login' ? 'С возвращением' : 'Создать аккаунт'}</h1>
            <p className="description">
              {mode === 'login'
                ? 'Войдите, чтобы продолжить общение.'
                : 'Зарегистрируйтесь, чтобы начать общение.'}
            </p>

            <form onSubmit={handleSubmit}>
              <label htmlFor="username">Имя пользователя</label>
              <input
                id="username"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                autoComplete="username"
                required
                maxLength={50}
              />

              <label htmlFor="password">Пароль</label>
              <input
                id="password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
                required
              />

              {error && <p className="message error" role="alert">{error}</p>}
              {notice && <p className="message notice" role="status">{notice}</p>}

              <button className="primary" type="submit" disabled={busy}>
                {busy ? 'Подождите…' : mode === 'login' ? 'Войти' : 'Зарегистрироваться'}
              </button>
            </form>

            <p className="switch">
              {mode === 'login' ? 'Нет аккаунта?' : 'Уже есть аккаунт?'}{' '}
              <button
                type="button"
                disabled={busy}
                onClick={() => changeMode(mode === 'login' ? 'register' : 'login')}
              >
                {mode === 'login' ? 'Создать' : 'Войти'}
              </button>
            </p>
          </>
        )}
      </section>
    </main>
  )
}