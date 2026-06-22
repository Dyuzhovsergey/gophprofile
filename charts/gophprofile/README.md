# GophProfile Helm Chart

Этот Helm Chart предназначен для деплоя GophProfile в Kubernetes.

Cоздан базовый каркас chart-а:

```text
charts/gophprofile/
├── Chart.yaml
├── values.yaml
├── templates/
│   └── _helpers.tpl
└── README.md
```

## Проверка chart-а

```bash
helm lint charts/gophprofile
```

Рендеринг templates:

```bash
helm template gophprofile charts/gophprofile
```

## ConfigMap и Secret

Chart создаёт ConfigMap с несекретной конфигурацией приложения:

```text
templates/configmap.yaml
```

По умолчанию Secret не создаётся:

```yaml
secret:
  create: false
  name: gophprofile-secret
```

В этом режиме перед установкой Chart в namespace должен существовать Secret с именем:

```text
gophprofile-secret
```

Он должен содержать ключи:

```text
GOPHPROFILE_DATABASE_DSN
GOPHPROFILE_S3_ACCESS_KEY
GOPHPROFILE_S3_SECRET_KEY
GOPHPROFILE_RABBITMQ_URL
```

Если нужно создавать Secret через Helm:

```yaml
secret:
  create: true
  name: gophprofile-secret
  databaseDSN: ""
  s3AccessKey: ""
  s3SecretKey: ""
  rabbitmqURL: ""
```
