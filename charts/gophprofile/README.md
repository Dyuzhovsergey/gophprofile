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

## Server и worker

Chart создаёт отдельные Deployment и Service для server и worker:

```text
templates/server-deployment.yaml
templates/server-service.yaml
templates/worker-deployment.yaml
templates/worker-service.yaml
```

Через `values.yaml` настраиваются:

- image repository и tag;
- imagePullPolicy;
- количество реплик;
- контейнерные и Service-порты;
- CPU и memory requests/limits;
- startup, liveness и readiness probes;
- termination grace period;
- rolling update strategy.

Server получает конфигурацию из ConfigMap и Secret:

```yaml
envFrom:
  - configMapRef:
      name: gophprofile-config
  - secretRef:
      name: gophprofile-secret
```


## Ingress, HPA и ServiceMonitor

Дополнительные Kubernetes-ресурсы по умолчанию выключены.

### Ingress

```yaml
ingress:
  enabled: false
  className: traefik
  host: gophprofile.local
```

Включение:

```bash
helm template gophprofile charts/gophprofile \
  --namespace gophprofile \
  --set ingress.enabled=true
```

### HPA

Server масштабируется по CPU и памяти, worker — по CPU.

```yaml
autoscaling:
  server:
    enabled: false
  worker:
    enabled: false
```

Когда HPA включён, поле `spec.replicas` в соответствующем Deployment не генерируется.



## NetworkPolicy, RBAC и SecurityContext

Chart поддерживает базовые настройки безопасности.

### RBAC

По умолчанию создаются:

- ServiceAccount;
- Role;
- RoleBinding.

Приложение не обращается к Kubernetes API, поэтому Role не содержит разрешений:

```yaml
rules: []
```

Токен ServiceAccount не монтируется внутрь контейнеров:

```yaml
automountServiceAccountToken: false
```

### SecurityContext

Server и worker запускаются:

- не от root;
- без privilege escalation;
- без Linux capabilities;
- с `RuntimeDefault` seccomp profile;
- с read-only root filesystem.

Для временных файлов монтируется writable volume `/tmp`.

### NetworkPolicy

NetworkPolicy по умолчанию выключен:

```yaml
networkPolicy:
  enabled: false
```

При включении создаются политики:

- входящий трафик к server от Traefik и monitoring namespace;
- входящий metrics-трафик к worker от monitoring namespace;
- исходящий трафик server/worker к DNS, PostgreSQL, RabbitMQ и MinIO.

## Миграции PostgreSQL

Для запуска миграций используется Helm hook:

```text
templates/migration-job.yaml
```

Job выполняется перед:

```text
helm install
helm upgrade
```

Аннотации hook-а:

```yaml
helm.sh/hook: pre-install,pre-upgrade
helm.sh/hook-weight: "-5"
helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded
```

После успешного выполнения Job удаляется. При ошибке Job остаётся в namespace, чтобы можно было проверить состояние и логи.

### Существующий Secret

По умолчанию используется уже существующий Secret:

```yaml
secret:
  create: false
  name: gophprofile-secret
```

Он должен содержать:

```text
GOPHPROFILE_DATABASE_DSN
```

Проверка:

```bash
kubectl get secret gophprofile-secret -n gophprofile
```

### Migrate image

Для локального Rancher Desktop используется image:

```text
docker.io/library/gophprofile-migrate:local
```

Сборка:

```bash
docker build \
  -f docker/migrate.Dockerfile \
  -t docker.io/library/gophprofile-migrate:local .
```

## Values-файлы окружений

Chart использует основной файл значений:

```text
values.yaml
```

Дополнительные values-файлы переопределяют только настройки конкретного окружения:

```text
values.local.yaml
values.prod.example.yaml
```

### Локальное окружение

`values.local.yaml` предназначен для Rancher Desktop:

- локальные images с тегом `local`;
- `imagePullPolicy: Never`;
- одна реплика server и worker;
- Traefik Ingress;
- host `gophprofile.local`;
- локальные PostgreSQL, RabbitMQ и MinIO;
- ServiceMonitor выключен, потому что в локальном кластере может отсутствовать Prometheus Operator;
- секреты берутся из заранее созданного `gophprofile-secret`.

Проверка:

```bash
helm lint charts/gophprofile \
  -f charts/gophprofile/values.local.yaml

helm template gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml
```


### Production example

`values.prod.example.yaml` — безопасный пример production-конфигурации:

- images из внешнего registry;
- фиксированные версии images;
- несколько реплик;
- HPA;
- TLS для Ingress;
- ServiceMonitor;
- увеличенные requests и limits.


Проверка production-render:

```bash
helm lint charts/gophprofile \
  -f charts/gophprofile/values.prod.example.yaml

helm template gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.prod.example.yaml
```

Порядок приоритетов Helm:

```text
values.yaml
  → values-файл окружения
    → параметры --set
```


## Подсказки после установки

Chart содержит:

```text
templates/NOTES.txt
```

После `helm install` или `helm upgrade` Helm показывает:

- имя release и namespace;
- состояние Kubernetes-ресурсов;
- адрес Ingress;
- команды port-forward;
- проверки `/live`, `/ready`, `/health` и `/metrics`;
- команды просмотра server и worker logs;
- состояние миграционного hook-а;
- проверки HPA, ServiceMonitor и NetworkPolicy.

Проверка без установки:

```bash
helm install gophprofile-notes charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --dry-run \
  --debug
```

После реальной установки подсказки можно показать повторно:

```bash
helm get notes gophprofile -n gophprofile
```

## Полное руководство по Helm-деплою

Подробная инструкция по установке, обновлению, rollback и удалению релиза находится в документе:

```text
docs/helm.md
```

Базовая локальная проверка:

```bash
helm lint charts/gophprofile \
  -f charts/gophprofile/values.local.yaml

helm template gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml
```

Локальная установка после подготовки namespace, Secret и dev-зависимостей:

```bash
helm upgrade --install gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --wait \
  --timeout 10m
```
