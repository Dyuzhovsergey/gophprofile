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

