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
- slog
- OpenTelemetry
- Jaeger
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
## Адреса сервисов
HTTP API	http://localhost:9090
Web upload	http://localhost:9090/web/upload
Web gallery	http://localhost:9090/web/gallery/user
PostgreSQL	localhost:5433
MinIO API	http://localhost:9000
MinIO Console	http://localhost:9001
RabbitMQ AMQP	localhost:5672
RabbitMQ Management UI	http://localhost:15672
Jaeger UI	http://localhost:16686
Jaeger OTLP HTTP	http://localhost:4318
Jaeger OTLP gRPC	http://localhost:4317


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
    "rabbitmq": "ok",
    "s3": "ok",
    "server": "ok"
  }
}
```
### Загрузка аватарки

#### Для PNG:
``` bash
curl -i -X POST http://localhost:9090/api/v1/avatars \
  -H "X-User-ID: user" \
  -F "file=@avatar.png;type=image/png"
```
#### Для JPEG:
``` bash
curl -i -X POST http://localhost:9090/api/v1/avatars \
  -H "X-User-ID: user" \
  -F "file=@avatar.jpg;type=image/jpeg"
```
#### Пример успешного ответа:
``` bash
{
  "id": "avatar-id",
  "user_id": "user",
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
  "user_id": "user",
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
curl -sS -L http://localhost:9090/api/v1/users/user/avatar \
  --output user_current_avatar.jpg
  ```

### Получение списка аватарок пользователя
``` bash
curl -i http://localhost:9090/api/v1/users/user/avatars
```
``` bash
Пример ответа:

{
  "user_id": "user",
  "avatars": [
    {
      "id": "avatar-id",
      "user_id": "user",
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
  -H "X-User-ID: user"
```
#### Ожидаемо:
``` bash
HTTP/1.1 204 No Content
```

### Удаление текущей аватарки пользователя
``` bash
curl -i -X DELETE http://localhost:9090/api/v1/users/user/avatar \
  -H "X-User-ID: user"
```
#### Ожидаемо:
``` bash
HTTP/1.1 204 No Content
```

## Prometheus metrics
Приложение отдаёт базовые Prometheus metrics на endpoint:

```bash
curl http://localhost:9090/metrics
```

## Prometheus

Prometheus доступен по адресу:
```bash
http://localhost:9092
```

## Grafana

Grafana доступна по адресу:

```bash
http://localhost:3000
```

### Проверка через браузер

#### Откройте:

http://localhost:9090/web/upload

#### Укажите:

User ID: user

#### Выберите изображение и нажмите загрузку.

#### После успешной загрузки можно открыть галерею:

http://localhost:9090/web/gallery/user

