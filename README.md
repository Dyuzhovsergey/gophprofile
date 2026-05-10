# GophProfile

GophProfile — микросервис для управления аватарками пользователей.

Сервис позволяет:

- загружать аватарку пользователя;
- получать оригинал аватарки по `avatar_id`;
- получать текущую аватарку пользователя по `user_id`;
- получать metadata аватарки;
- получать список аватарок пользователя;
- получать thumbnails `100x100` и `300x300`;
- мягко удалять аватарку;
- асинхронно обрабатывать изображения через RabbitMQ worker.

## Технологии

- Go
- Chi
- Zap
- PostgreSQL
- Goose migrations
- MinIO / S3
- RabbitMQ
- Docker Compose

## Структура проекта

```text
cmd/
  server/        HTTP-сервер
  worker/        Фоновый worker

internal/
  broker/        Работа с RabbitMQ
  config/        Конфигурация приложения
  domain/        Доменные сущности и ошибки
  handlers/      HTTP handlers и router
  logger/        Zap logger
  middleware/    HTTP middleware
  repository/    PostgreSQL и S3 repositories
  services/      Бизнес-логика
  worker/        Обработчики фоновых событий

migrations/      SQL-миграции PostgreSQL
docker/          Dockerfile-ы server и worker
web/             Статические файлы
```

## Локальный запуск через Docker Compose

### Docker Compose поднимает:

PostgreSQL;
MinIO;
RabbitMQ;
goose migration;
server;
worker.

### Запуск:
``` bash
docker compose up --build -d
```

### Остановить инфраструктуру:
``` bash
docker compose down -v
```
## Проверка API
### Health check
``` bash
curl -i http://localhost:9090/health
```
#### Ожидаемый ответ:
``` bash
HTTP/1.1 200 OK
```

#### Пример тела:
``` bash
{
  "status": "ok",
  "details": {
    "postgres": "ok",
    "server": "ok"
  }
}
```
### Загрузка аватарки

#### Для PNG:
``` bash
curl -i -X POST http://localhost:9090/api/v1/avatars \
  -H "X-User-ID: sergey" \
  -F "file=@avatar.png;type=image/png"
```
#### Для JPEG:
``` bash
curl -i -X POST http://localhost:9090/api/v1/avatars \
  -H "X-User-ID: sergey" \
  -F "file=@avatar.jpg;type=image/jpeg"
```
#### Пример успешного ответа:
``` bash
{
  "id": "avatar-id",
  "user_id": "sergey",
  "url": "/api/v1/avatars/avatar-id",
  "status": "pending",
  "created_at": "2026-05-10T08:00:00Z"
}
```
### Проверка обработки в PostgreSQL
``` bash
docker exec -it gophprofile-postgres psql -U gophprofile -d gophprofile -c \
"select id, s3_key, width, height, thumbnail_s3_keys, processing_status from avatars order by created_at desc limit 5;"
```
#### После успешной обработки worker-ом ожидаемо:
 ``` bash
processing_status = completed
width > 0
height > 0
thumbnail_s3_keys содержит 100x100 и 300x300
```

#### Получение оригинала аватарки
``` bash
curl -sS -L http://localhost:9090/api/v1/avatars/<avatar_id> \
  --output original.jpg
  ```
#### Получение thumbnail 100x100
``` bash
curl -sS -L "http://localhost:9090/api/v1/avatars/<avatar_id>?size=100x100" \
  --output avatar_100x100.jpg
  ```
#### Получение thumbnail 300x300
``` bash
curl -sS -L "http://localhost:9090/api/v1/avatars/<avatar_id>?size=300x300" \
  --output avatar_300x300.jpg
```

### Получение metadata аватарки
``` bash
curl -i http://localhost:9090/api/v1/avatars/<avatar_id>/metadata
```
#### Пример ответа:
``` bash
{
  "id": "avatar-id",
  "user_id": "sergey",
  "file_name": "avatar.png",
  "mime_type": "image/png",
  "size": 12345,
  "dimensions": {
    "width": 303,
    "height": 295
  },
  "thumbnails": [
    {
      "size": "100x100",
      "url": "/api/v1/avatars/avatar-id?size=100x100"
    },
    {
      "size": "300x300",
      "url": "/api/v1/avatars/avatar-id?size=300x300"
    }
  ],
  "created_at": "2026-05-10T08:00:00Z",
  "updated_at": "2026-05-10T08:00:01Z"
}
```

### Получение текущей аватарки пользователя
``` bash
curl -sS -L http://localhost:9090/api/v1/users/sergey/avatar \
  --output sergey_current_avatar.jpg
  ```

### Получение списка аватарок пользователя
``` bash
curl -i http://localhost:9090/api/v1/users/sergey/avatars
```
``` bash
Пример ответа:

{
  "user_id": "sergey",
  "avatars": [
    {
      "id": "avatar-id",
      "user_id": "sergey",
      "file_name": "avatar.png",
      "mime_type": "image/png",
      "size": 12345,
      "dimensions": {
        "width": 303,
        "height": 295
      },
      "thumbnails": [
        {
          "size": "100x100",
          "url": "/api/v1/avatars/avatar-id?size=100x100"
        },
        {
          "size": "300x300",
          "url": "/api/v1/avatars/avatar-id?size=300x300"
        }
      ],
      "created_at": "2026-05-10T08:00:00Z",
      "updated_at": "2026-05-10T08:00:01Z"
    }
  ]
}
```

### Мягкое удаление аватарки по avatar_id
``` bash
curl -i -X DELETE http://localhost:9090/api/v1/avatars/<avatar_id> \
  -H "X-User-ID: sergey"
```
#### Ожидаемо:
``` bash
HTTP/1.1 204 No Content
```

### Удаление текущей аватарки пользователя
``` bash
curl -i -X DELETE http://localhost:9090/api/v1/users/sergey/avatar \
  -H "X-User-ID: sergey"
```
#### Ожидаемо:
``` bash
HTTP/1.1 204 No Content
```
