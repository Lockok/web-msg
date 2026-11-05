# Web client

React + TypeScript + Vite-клиент для web-msg.

## Локальный запуск

Из папки `frontend`:

```powershell
npm.cmd install
npm.cmd run dev
```

В другом терминале из корня проекта:

```powershell
go run ./cmd/server
```

Открой адрес, показанный Vite. Прокси разработки направляет запросы
на Go-сервер по адресу `localhost:8080`.

## Проверки

```powershell
npm.cmd run build
npm.cmd run lint
npm.cmd audit
```

## Ограничения

Токен хранится в памяти и теряется при обновлении страницы.
WebSocket-прокси предназначен только для локальной разработки.

В инструментах разработки остаются предупреждения `npm audit`;
перед обновлением необходимо проверить совместимость зависимостей.