# Kubernetes manifests

В этой директории будут храниться обычные Kubernetes-манифесты проекта GophProfile.

## Структура

```text
k8s/
├── base/
│   └── .gitkeep
├── dev/
│   └── .gitkeep
└── README.md
```

# Правила для Kubernetes-манифестов

## Naming conventions и labels

Для всех Kubernetes-ресурсов проекта используем единый namespace gophprofile

Базовые labels:

1. app.kubernetes.io/name: gophprofile
2. app.kubernetes.io/instance: gophprofile
3. app.kubernetes.io/component: component
4. app.kubernetes.io/part-of: gophprofile
5. app.kubernetes.io/managed-by: kubectl

Значение app.kubernetes.io/component зависит от типа ресурса:

- Компонент	Label
- Namespace	namespace
- Server	server
- Worker	worker
- Migration Job	migration
- PostgreSQL dev dependency	postgres
- RabbitMQ dev dependency	rabbitmq
- MinIO dev dependency	minio

## ConfigMap

Несекретная конфигурация приложения хранится в k8s/base/configmap.yaml

В нём хранятся только значения, которые можно безопасно держать в Git:

- log level;
- адрес HTTP-сервера внутри контейнера;
- адрес metrics endpoint worker-а;
- максимальный размер загружаемого файла;
- S3 endpoint, region, bucket и path-style режим;
- имена RabbitMQ exchange, queues и routing keys;
- feature flag для OpenTelemetry;
- OTLP exporter endpoint.

