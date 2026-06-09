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

## Secret

Секретная конфигурация приложения описана в example-файле k8s/base/secret.example.yaml
В нём находятся чувствительные env-переменные:

* GOPHPROFILE_DATABASE_DSN;
* GOPHPROFILE_S3_ACCESS_KEY;
* GOPHPROFILE_S3_SECRET_KEY;
* GOPHPROFILE_RABBITMQ_URL.

## Docker images для Kubernetes

В проекте server и worker собираются отдельными Dockerfile-ами:

```text
docker/server.Dockerfile
docker/worker.Dockerfile
```

Для Kubernetes используем отдельные образы:

```text
gophprofile-server:local
gophprofile-worker:local
```

### Сборка образа server

```bash
docker build -f docker/server.Dockerfile -t gophprofile-server:local .
```

### Сборка образа worker

```bash
docker build -f docker/worker.Dockerfile -t gophprofile-worker:local .
```

### Проверка локальных образов

```bash
docker images | grep gophprofile
```

Ожидаемо должны появиться два образа:

```text
gophprofile-server   local
gophprofile-worker   local
```

## Ingress

Для внешнего HTTP-доступа используется Kubernetes Ingress:

```text
k8s/base/ingress.yaml
```

Локальный host:

gophprofile.local


## Liveness и readiness probes

В приложении используются отдельные endpoints для Kubernetes probes:

| Endpoint | Назначение |
|---|---|
| `/live` | Проверяет, что процесс жив и HTTP-server отвечает |
| `/ready` | Проверяет, что приложение готово принимать трафик |
| `/health` | Старый общий healthcheck для ручной проверки и совместимости |

Для server:

- `livenessProbe` использует `/live`;
- `readinessProbe` использует `/ready`.

Для worker:

- probes доступны на metrics-порту `9091`;
- `livenessProbe` использует `/live`;
- `readinessProbe` использует `/ready`.

## HorizontalPodAutoscaler

Для server используется HPA:

```text
k8s/base/server-hpa.yaml
```

Он масштабирует Deployment gophprofile-server по CPU и memory:

CPU target: 70%;
memory target: 80%;
minReplicas: 1;
maxReplicas: 3.


Для worker используется HPA:

```text
k8s/base/worker-hpa.yaml
```

Он масштабирует Deployment gophprofile-server по CPU:
Текущая dev-стратегия:

minReplicas: 1;
maxReplicas: 3;
CPU target utilization: 70%.


## ServiceMonitor для server metrics

Server отдаёт Prometheus-метрики на endpoint:

```text
/metrics
```

В Kubernetes для автоматического обнаружения server metrics используется:
k8s/base/server-servicemonitor.yaml

ServiceMonitor выбирает Service gophprofile-server по labels:
app.kubernetes.io/name: gophprofile
app.kubernetes.io/instance: gophprofile
app.kubernetes.io/component: server

И собирает метрики с порта Service:
port: metrics
path: /metrics

## ServiceMonitor для worker metrics

Worker отдаёт Prometheus-метрики на отдельном metrics-порту:

```text
9091
```
Endpoint метрик:

/metrics

Для автоматического обнаружения worker metrics используется манифест:
k8s/base/worker-servicemonitor.yaml

ServiceMonitor выбирает Service gophprofile-worker по labels:
app.kubernetes.io/name: gophprofile
app.kubernetes.io/instance: gophprofile
app.kubernetes.io/component: worker

И собирает метрики с порта Service:
port: metrics
path: /metrics



## NetworkPolicy для входящего трафика server

Для ограничения входящего трафика к server Pod используется манифест:

```text
k8s/base/networkpolicy-ingress.yaml
```

Политика выбирает server Pod по labels:

```yaml
app.kubernetes.io/name: gophprofile
app.kubernetes.io/instance: gophprofile
app.kubernetes.io/component: server
```

Разрешён входящий трафик:
- от ingress-controller Traefik из namespace `kube-system`;
- от Prometheus из namespace `monitoring`, если позже будет установлен Prometheus Operator.
