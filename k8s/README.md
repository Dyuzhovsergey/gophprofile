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

## Traefik IngressRoute и ограничение размера запроса

Для внешнего HTTP-доступа в локальном Kubernetes используется Traefik:

```text
k8s/base/ingress.yaml
```
Манифест создаёт два Traefik CRD-ресурса:

Middleware — ограничивает размер тела HTTP-запроса;
IngressRoute — направляет запросы в gophprofile-server.

Максимальный размер request body:

10485760 байт = 10 MiB

Маршрут:

Host: gophprofile.local
PathPrefix: /

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

Он масштабирует Deployment gophprofile-worker по CPU и memory:
Текущая dev-стратегия:

minReplicas: 1;
maxReplicas: 3;
CPU target utilization: 70%.
memory target utilization: 80%.


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



## NetworkPolicy для входящего трафика server и worker

Для ограничения входящего трафика к server и worker используется манифест:

```text
k8s/base/networkpolicy-ingress.yaml
```
Он создаёт две политики:

gophprofile-server-ingress;
gophprofile-worker-ingress.

Для server разрешён входящий трафик:

от Traefik из namespace kube-system на порт 8080;
от Prometheus из namespace monitoring на порт 8080.

Для worker разрешён входящий трафик только от namespace monitoring
на metrics-порт 9091.

## NetworkPolicy для исходящего трафика server и worker

Для ограничения исходящего трафика от application Pod используется манифест:

```text
k8s/base/networkpolicy-egress.yaml
```

Политика выбирает Pod-ы:

```yaml
app.kubernetes.io/component: server
app.kubernetes.io/component: worker
```

Разрешён исходящий трафик:

- DNS в namespace `kube-system`, порт `53` TCP/UDP;
- PostgreSQL, порт `5432`;
- RabbitMQ AMQP, порт `5672`;
- MinIO S3 API, порт `9000`.


## ServiceAccount и RBAC

Для server и worker используется отдельный ServiceAccount:

```text
gophprofile-app
```

Манифест:

```text
k8s/base/rbac.yaml
```

## SecurityContext для server и worker

Для server и worker настроены Pod-level и container-level securityContext.

Pod-level настройки:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 10001
  runAsGroup: 10001
  fsGroup: 10001
  seccompProfile:
    type: RuntimeDefault
```

Container-level настройки:

```yaml
securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

- контейнер запускается не от root;
- Kubernetes API token не монтируется внутрь контейнера;
- privilege escalation запрещён;
- Linux capabilities сброшены;
- root filesystem доступен только на чтение;
- для временных файлов используется отдельный writable volume `/tmp`.


## PodDisruptionBudget и rolling update strategy

Для server и worker настроены:

- `PodDisruptionBudget`;
- rolling update strategy в Deployment.

Манифесты:

```text
k8s/base/server-pdb.yaml
k8s/base/worker-pdb.yaml
```

Для server и worker используется PDB:

```yaml
minAvailable: 1
```

Это означает, что при добровольных disruptions Kubernetes должен сохранить минимум один доступный Pod выбранного компонента.

В Deployment-ах настроена стратегия обновления:

```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxUnavailable: 0
    maxSurge: 1
```

Что это означает:

- `maxUnavailable: 0` — во время обновления нельзя оставлять компонент без доступного Pod;
- `maxSurge: 1` — Kubernetes может временно создать один дополнительный Pod;
- новый Pod должен пройти readiness probe, прежде чем старый Pod будет остановлен.

## Полный деплой GophProfile через kubectl

Этот раздел описывает развёртывание GophProfile в локальном Kubernetes-кластере Rancher Desktop с чистого namespace.

В локальном окружении внутри Kubernetes запускаются:

* PostgreSQL;
* RabbitMQ;
* MinIO;
* migration Job;
* GophProfile server;
* GophProfile worker.

Обычные Kubernetes-манифесты находятся в директориях:

```text
k8s/base/
k8s/dev/
```

Манифесты из `k8s/dev` предназначены только для локальной разработки и не являются production-ready.

### Предварительные требования

Должны быть установлены и запущены:

* Rancher Desktop;
* Kubernetes в Rancher Desktop;
* контейнерный движок Moby/dockerd;
* `docker`;
* `kubectl`.

Проверка Docker:

```bash
docker version
```

Проверка текущего Kubernetes context:

```bash
kubectl config current-context
```

Ожидаемо:

```text
rancher-desktop
```

Если выбран другой context:

```bash
kubectl config use-context rancher-desktop
```

Проверка кластера:

```bash
kubectl cluster-info
kubectl get nodes
```

Node должен находиться в состоянии:

```text
Ready
```

### 1. Сборка локальных Docker-образов

Kubernetes Deployment использует локальные images с политикой:

```yaml
imagePullPolicy: Never
```

Поэтому images должны быть собраны в Docker Rancher Desktop до запуска Pod-ов.

Сборка server:

```bash
docker build \
  -f docker/server.Dockerfile \
  -t docker.io/library/gophprofile-server:local .
```

Сборка worker:

```bash
docker build \
  -f docker/worker.Dockerfile \
  -t docker.io/library/gophprofile-worker:local .
```

Сборка мигратора:

```bash
docker build \
  -f docker/migrate.Dockerfile \
  -t docker.io/library/gophprofile-migrate:local .
```

Проверка:

```bash
docker images | grep gophprofile
```

Ожидаемые images:

```text
gophprofile-server    local
gophprofile-worker    local
gophprofile-migrate   local
```

### 2. Чистое развёртывание

Удаление namespace полностью удаляет ресурсы и данные локальных PVC.

Использовать эту команду следует только для полного сброса dev-окружения:

```bash
kubectl delete namespace gophprofile --ignore-not-found
```

Дождаться удаления namespace:

```bash
kubectl wait \
  --for=delete namespace/gophprofile \
  --timeout=180s 2>/dev/null || true
```

Создать namespace заново:

```bash
kubectl apply -f k8s/base/namespace.yaml
kubectl get namespace gophprofile
```

### 3. ConfigMap

ConfigMap содержит несекретную конфигурацию server и worker:

```bash
kubectl apply -f k8s/base/configmap.yaml
```

Проверка:

```bash
kubectl get configmap gophprofile-config -n gophprofile
kubectl describe configmap gophprofile-config -n gophprofile
```

В Kubernetes должны использоваться DNS-имена Service, а не `localhost`:

```text
gophprofile-postgres
gophprofile-rabbitmq
gophprofile-minio
```

### 4. Локальный Secret

Создать локальный файл из безопасного примера:

```bash
cp k8s/base/secret.example.yaml \
   k8s/base/secret.local.yaml
```

Открыть его:

```bash
nano k8s/base/secret.local.yaml
```

Для текущих dev-манифестов значения должны выглядеть примерно так:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gophprofile-secret
  namespace: gophprofile
type: Opaque
stringData:
  GOPHPROFILE_DATABASE_DSN: "postgres://gophprofile:gophprofile@gophprofile-postgres:5432/gophprofile?sslmode=disable"
  GOPHPROFILE_S3_ACCESS_KEY: "minioadmin"
  GOPHPROFILE_S3_SECRET_KEY: "minioadmin"
  GOPHPROFILE_RABBITMQ_URL: "amqp://gophprofile:gophprofile@gophprofile-rabbitmq:5672/"
```

Применить Secret:

```bash
kubectl apply -f k8s/base/secret.local.yaml
```

Проверка наличия Secret без вывода его содержимого:

```bash
kubectl get secret gophprofile-secret -n gophprofile
```

Файл `secret.local.yaml` нельзя добавлять в Git.

### 5. ServiceAccount и RBAC

Создать отдельный ServiceAccount и минимальные RBAC-настройки:

```bash
kubectl apply -f k8s/base/rbac.yaml
```

Проверка:

```bash
kubectl get serviceaccount,role,rolebinding -n gophprofile
```

Проверка отсутствия лишних прав:

```bash
kubectl auth can-i list pods \
  --as=system:serviceaccount:gophprofile:gophprofile-app \
  -n gophprofile
```

Ожидаемый ответ:

```text
no
```

### 6. PostgreSQL, RabbitMQ и MinIO

Применить dev-зависимости:

```bash
kubectl apply -f k8s/dev/postgres.yaml
kubectl apply -f k8s/dev/rabbitmq.yaml
kubectl apply -f k8s/dev/minio.yaml
```

Посмотреть создаваемые ресурсы:

```bash
kubectl get pods,svc,pvc,jobs -n gophprofile
```

Дождаться готовности PostgreSQL:

```bash
kubectl rollout status \
  deployment/gophprofile-postgres \
  -n gophprofile \
  --timeout=300s
```

Дождаться готовности RabbitMQ:

```bash
kubectl rollout status \
  deployment/gophprofile-rabbitmq \
  -n gophprofile \
  --timeout=300s
```

Дождаться готовности MinIO:

```bash
kubectl rollout status \
  deployment/gophprofile-minio \
  -n gophprofile \
  --timeout=300s
```

Дождаться создания bucket в MinIO:

```bash
kubectl wait \
  --for=condition=complete \
  job/gophprofile-minio-init \
  -n gophprofile \
  --timeout=300s
```

Проверить логи инициализации MinIO:

```bash
kubectl logs \
  job/gophprofile-minio-init \
  -n gophprofile
```

### 7. Миграции PostgreSQL

Применить migration Job:

```bash
kubectl apply -f k8s/base/migration-job.yaml
```

Дождаться выполнения:

```bash
kubectl wait \
  --for=condition=complete \
  job/gophprofile-migrations \
  -n gophprofile \
  --timeout=300s
```

Проверить логи:

```bash
kubectl logs \
  job/gophprofile-migrations \
  -n gophprofile
```

Ожидаемое сообщение:

```text
migrations completed successfully
```

Проверить таблицы PostgreSQL:

```bash
kubectl exec \
  deployment/gophprofile-postgres \
  -n gophprofile \
  -- psql -U gophprofile -d gophprofile -c '\dt'
```

В списке должны присутствовать таблицы:

```text
avatars
outbox_events
goose_db_version
```

Чтобы повторно запустить migration Job, сначала удалить старый Job:

```bash
kubectl delete job \
  gophprofile-migrations \
  -n gophprofile \
  --ignore-not-found

kubectl apply -f k8s/base/migration-job.yaml
```

### 8. Service для server и worker

Создать внутренние Kubernetes Service:

```bash
kubectl apply -f k8s/base/server-service.yaml
kubectl apply -f k8s/base/worker-service.yaml
```

Проверка:

```bash
kubectl get services -n gophprofile
```

Ожидаемые Service:

```text
gophprofile-server
gophprofile-worker
gophprofile-postgres
gophprofile-rabbitmq
gophprofile-minio
```

### 9. Deployment server и worker

Применить Deployment server:

```bash
kubectl apply -f k8s/base/server-deployment.yaml
```

Применить Deployment worker:

```bash
kubectl apply -f k8s/base/worker-deployment.yaml
```

Дождаться server:

```bash
kubectl rollout status \
  deployment/gophprofile-server \
  -n gophprofile \
  --timeout=300s
```

Дождаться worker:

```bash
kubectl rollout status \
  deployment/gophprofile-worker \
  -n gophprofile \
  --timeout=300s
```

Проверить Pod-ы:

```bash
kubectl get pods -n gophprofile
```

Server и worker должны находиться в состоянии:

```text
1/1 Running
```

Проверить логи server:

```bash
kubectl logs \
  -l app.kubernetes.io/component=server \
  -n gophprofile \
  --tail=100
```

Проверить логи worker:

```bash
kubectl logs \
  -l app.kubernetes.io/component=worker \
  -n gophprofile \
  --tail=100
```

### 10. Ingress

Проверить доступные IngressClass:

```bash
kubectl get ingressclass
```

Для Rancher Desktop используется Traefik.

Применить Ingress:

```bash
kubectl apply -f k8s/base/ingress.yaml
```

Проверка:

```bash
kubectl get middleware,ingressroute \
  -n gophprofile

kubectl describe middleware \
  gophprofile-upload-body-limit \
  -n gophprofile

kubectl describe ingressroute \
  gophprofile-server \
  -n gophprofile
```

Добавить локальный host, если его ещё нет:

```bash
grep -q "gophprofile.local" /etc/hosts || \
  echo "127.0.0.1 gophprofile.local" | sudo tee -a /etc/hosts
```

Проверить:

```bash
getent hosts gophprofile.local
```

### 11. HPA

Применить HPA для server и worker:

```bash
kubectl apply -f k8s/base/server-hpa.yaml
kubectl apply -f k8s/base/worker-hpa.yaml
```

Проверка:

```bash
kubectl get hpa -n gophprofile
kubectl top pods -n gophprofile
```

Если в колонке `TARGETS` временно отображается `<unknown>`, нужно подождать, пока Metrics Server соберёт первые метрики.

Проверка Metrics Server:

```bash
kubectl top nodes
```

### 12. PodDisruptionBudget

Применить PDB:

```bash
kubectl apply -f k8s/base/server-pdb.yaml
kubectl apply -f k8s/base/worker-pdb.yaml
```

Проверка:

```bash
kubectl get pdb -n gophprofile
```

При одной реплике значение `ALLOWED DISRUPTIONS` может быть равно `0`. Это ожидаемо для `minAvailable: 1`.

### 13. NetworkPolicy

NetworkPolicy следует применять после того, как базовая работа приложения уже проверена.

Применить входящую политику:

```bash
kubectl apply -f k8s/base/networkpolicy-ingress.yaml
```

Применить исходящую политику:

```bash
kubectl apply -f k8s/base/networkpolicy-egress.yaml
```

Проверка:

```bash
kubectl get networkpolicy -n gophprofile
kubectl describe networkpolicy -n gophprofile
```

NetworkPolicy реально ограничивает трафик только при поддержке сетевых политик CNI-плагином кластера.

Если после применения server или worker потеряли доступ к зависимостям, выполнить откат:

```bash
kubectl delete networkpolicy \
  gophprofile-server-ingress \
  gophprofile-worker-ingress \
  gophprofile-app-egress \
  -n gophprofile \
  --ignore-not-found
```

Затем проверить labels зависимостей:

```bash
kubectl get pods -n gophprofile --show-labels
```

### 14. Kubernetes monitoring и ServiceMonitor

Для мониторинга Kubernetes используется отдельный Helm-релиз
`kube-prometheus-stack`:

```text
release: monitoring
namespace: monitoring
```

### 15. Проверка через Ingress

Liveness:

```bash
curl -i http://gophprofile.local/live
```

Readiness:

```bash
curl -i http://gophprofile.local/ready
```

Общий healthcheck:

```bash
curl -i http://gophprofile.local/health
```

Prometheus metrics:

```bash
curl -i http://gophprofile.local/metrics
```

Web-интерфейс:

```text
http://gophprofile.local/web/upload
```

### 16. Проверка через port-forward

Если Ingress недоступен, запустить:

```bash
kubectl port-forward \
  svc/gophprofile-server \
  8080:80 \
  -n gophprofile
```

Команда создаёт временный туннель:

```text
localhost:8080
  → Service gophprofile-server:80
  → server Pod:8080
```

В другом терминале:

```bash
curl -i http://localhost:8080/live
curl -i http://localhost:8080/ready
curl -i http://localhost:8080/health
curl -i http://localhost:8080/metrics
```

Worker metrics:

```bash
kubectl port-forward \
  svc/gophprofile-worker \
  9091:9091 \
  -n gophprofile
```

В другом терминале:

```bash
curl -i http://localhost:9091/live
curl -i http://localhost:9091/ready
curl -i http://localhost:9091/metrics
```

`port-forward` работает только пока соответствующая команда запущена в терминале.

### 17. End-to-end проверка загрузки

Убедиться, что в корне проекта есть тестовое изображение:

```bash
ls -lh avatar.jpg
```

Загрузить аватарку:

```bash
curl -i \
  -X POST \
  http://gophprofile.local/api/v1/avatars \
  -H "X-User-ID: sergey" \
  -F "file=@avatar.jpg"
```

Ожидаемый HTTP-статус:

```text
201 Created
```

Проверить работу worker:

```bash
kubectl logs \
  -l app.kubernetes.io/component=worker \
  -n gophprofile \
  --tail=100
```

Проверить историю пользователя:

```bash
curl -i \
  http://gophprofile.local/api/v1/users/sergey/avatars
```

### 18. Итоговое состояние ресурсов

Проверить основные ресурсы:

```bash
kubectl get all -n gophprofile
```

Дополнительные ресурсы:

```bash
kubectl get \
  ingressroute,middleware,hpa,pdb,networkpolicy \
  -n gophprofile
```

PVC:

```bash
kubectl get pvc -n gophprofile
```

Ожидается, что основные Pod-ы находятся в `Running`:

```text
gophprofile-postgres
gophprofile-rabbitmq
gophprofile-minio
gophprofile-server
gophprofile-worker
```

Job-ы после успешного выполнения могут находиться в `Completed`.

### 19. Полезные диагностические команды

События namespace:

```bash
kubectl get events \
  -n gophprofile \
  --sort-by=.lastTimestamp | tail -50
```

Описание server Pod:

```bash
kubectl describe pod \
  -l app.kubernetes.io/component=server \
  -n gophprofile
```

Описание worker Pod:

```bash
kubectl describe pod \
  -l app.kubernetes.io/component=worker \
  -n gophprofile
```

Логи server:

```bash
kubectl logs \
  -l app.kubernetes.io/component=server \
  -n gophprofile \
  --tail=100
```

Логи worker:

```bash
kubectl logs \
  -l app.kubernetes.io/component=worker \
  -n gophprofile \
  --tail=100
```

### 20. Частые проблемы

#### `ErrImageNeverPull` или `ImagePullBackOff`

Причина: image отсутствует в Docker Rancher Desktop.

Проверка:

```bash
docker images | grep gophprofile
```

Решение: повторно собрать нужный image с точным именем и тегом.

#### Pod находится в `Pending`

Посмотреть причину:

```bash
kubectl describe pod <pod-name> -n gophprofile
```

И события:

```bash
kubectl get events \
  -n gophprofile \
  --sort-by=.lastTimestamp | tail -30
```

#### `CrashLoopBackOff`

Посмотреть текущие логи:

```bash
kubectl logs <pod-name> -n gophprofile
```

Посмотреть логи предыдущего запуска контейнера:

```bash
kubectl logs <pod-name> -n gophprofile --previous
```

#### `relation "..." does not exist`

Причина: не выполнены миграции PostgreSQL.

Проверить migration Job:

```bash
kubectl get jobs -n gophprofile
kubectl logs job/gophprofile-migrations -n gophprofile
```

#### `no matches for kind "ServiceMonitor"`

Причина: отсутствует Prometheus Operator и CRD ServiceMonitor.

В локальном кластере пропустить применение ServiceMonitor-манифестов.

#### `connection refused` к RabbitMQ

Проверить состояние RabbitMQ:

```bash
kubectl get pods \
  -l app.kubernetes.io/component=rabbitmq \
  -n gophprofile

kubectl logs \
  -l app.kubernetes.io/component=rabbitmq \
  -n gophprofile \
  --tail=100
```

Проверить Service endpoints:

```bash
kubectl get endpoints gophprofile-rabbitmq -n gophprofile
```

### 21. Очистка локального окружения

Удалить только приложение, сохранив зависимости и данные:

```bash
kubectl delete \
  -f k8s/base/server-deployment.yaml \
  -f k8s/base/server-service.yaml \
  -f k8s/base/worker-deployment.yaml \
  -f k8s/base/worker-service.yaml \
  -f k8s/base/ingress.yaml \
  --ignore-not-found
```

Полностью удалить dev-окружение вместе с PVC и данными:

```bash
kubectl delete namespace gophprofile
```
