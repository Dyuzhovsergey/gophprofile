# Развёртывание GophProfile через Helm

Этот документ описывает проверку, установку, обновление, rollback и удаление Helm-релиза GophProfile.

Helm Chart находится в директории:

```text
charts/gophprofile/
```

Основные файлы:

```text
charts/gophprofile/
├── Chart.yaml
├── values.yaml
├── values.local.yaml
├── values.prod.example.yaml
├── templates/
└── README.md
```

## Что разворачивает Helm Chart

Chart может создавать:

* ConfigMap приложения;
* Secret или ссылку на существующий Secret;
* Deployment и Service для server;
* Deployment и Service для worker;
* Ingress;
* HPA для server и worker;
* ServiceMonitor для server и worker;
* NetworkPolicy;
* ServiceAccount, Role и RoleBinding;
* migration Job как Helm hook.

Локальные PostgreSQL, RabbitMQ и MinIO находятся в `k8s/dev` и не входят в Helm Chart.

## Предварительные требования

Должны быть установлены:

* Rancher Desktop;
* Kubernetes;
* Docker/Moby;
* `kubectl`;
* Helm 3.

Проверка:

```bash
docker version
kubectl config current-context
kubectl get nodes
helm version
```

Для локального запуска ожидаемый Kubernetes context:

```text
rancher-desktop
```

При необходимости:

```bash
kubectl config use-context rancher-desktop
```

## Values-файлы

Основные значения находятся в:

```text
charts/gophprofile/values.yaml
```

Локальные переопределения:

```text
charts/gophprofile/values.local.yaml
```

Безопасный production-пример:

```text
charts/gophprofile/values.prod.example.yaml
```

Приоритет значений:

```text
values.yaml
  → дополнительный values-файл
    → параметры --set
```

Например:

```bash
helm template gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --set server.replicaCount=2
```

Параметр `--set` имеет самый высокий приоритет.

## Проверка Chart перед установкой

Проверка основного Chart:

```bash
helm lint charts/gophprofile
```

Проверка локальных values:

```bash
helm lint charts/gophprofile \
  -f charts/gophprofile/values.local.yaml
```

Проверка production example:

```bash
helm lint charts/gophprofile \
  -f charts/gophprofile/values.prod.example.yaml
```

Рендеринг локального варианта:

```bash
helm template gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  > /tmp/gophprofile-local.yaml
```

Просмотр создаваемых ресурсов:

```bash
grep "^kind:" /tmp/gophprofile-local.yaml
```

Проверка встроенных Kubernetes-ресурсов:

```bash
kubectl apply --dry-run=client \
  -f /tmp/gophprofile-local.yaml
```

В локальных values ServiceMonitor выключен, поэтому отсутствие его CRD не мешает этой проверке.

Проверка Helm без реальной установки:

```bash
helm upgrade --install gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --dry-run \
  --debug
```

## Подготовка локальных Docker-образов

Локальные values используют:

```yaml
imagePullPolicy: Never
```

Поэтому images должны существовать в Docker Rancher Desktop.

Сборка:

```bash
docker build \
  -f docker/server.Dockerfile \
  -t docker.io/library/gophprofile-server:local .

docker build \
  -f docker/worker.Dockerfile \
  -t docker.io/library/gophprofile-worker:local .

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

## Подготовка namespace и Secret

Migration hook запускается до обычных ресурсов Chart. Поэтому namespace, PostgreSQL и существующий Secret должны быть готовы до `helm install`.

Создать namespace:

```bash
kubectl apply -f k8s/base/namespace.yaml
```

Создать локальный Secret из примера:

```bash
cp k8s/base/secret.example.yaml \
   k8s/base/secret.local.yaml
```

Заполнить `k8s/base/secret.local.yaml` и применить:

```bash
kubectl apply -f k8s/base/secret.local.yaml
```

Проверить:

```bash
kubectl get secret gophprofile-secret -n gophprofile
```

Файл `secret.local.yaml` нельзя коммитить в Git.

## Стратегия Secret

По умолчанию используется существующий Secret:

```yaml
secret:
  create: false
  name: gophprofile-secret
```

Он должен содержать ключи:

```text
GOPHPROFILE_DATABASE_DSN
GOPHPROFILE_S3_ACCESS_KEY
GOPHPROFILE_S3_SECRET_KEY
GOPHPROFILE_RABBITMQ_URL
```

Для production Secret рекомендуется создавать через:

* CI/CD;
* External Secrets;
* Sealed Secrets;
* Vault или другой secret manager;
* ручное создание администратором кластера.

Реальные секреты нельзя хранить в:

```text
values.yaml
values.local.yaml
values.prod.example.yaml
```

Chart также поддерживает:

```yaml
secret:
  create: true
```

Но секретные значения в таком случае должны передаваться через защищённый внешний values-файл или CI/CD, а не храниться в репозитории.

## Запуск локальных зависимостей

Применить PostgreSQL:

```bash
kubectl apply -f k8s/dev/postgres.yaml
```

Применить RabbitMQ:

```bash
kubectl apply -f k8s/dev/rabbitmq.yaml
```

Применить MinIO и Job создания bucket:

```bash
kubectl apply -f k8s/dev/minio.yaml
```

Дождаться готовности:

```bash
kubectl rollout status \
  deployment/gophprofile-postgres \
  -n gophprofile \
  --timeout=300s

kubectl rollout status \
  deployment/gophprofile-rabbitmq \
  -n gophprofile \
  --timeout=300s

kubectl rollout status \
  deployment/gophprofile-minio \
  -n gophprofile \
  --timeout=300s
```

Дождаться MinIO init Job:

```bash
kubectl wait \
  --for=condition=complete \
  job/gophprofile-minio-init \
  -n gophprofile \
  --timeout=300s
```

Проверить:

```bash
kubectl get pods,svc,pvc,jobs -n gophprofile
```

## Переход с kubectl-манифестов на Helm

В namespace могут уже существовать ресурсы с теми же именами, созданные через `kubectl`.

Helm не может автоматически принять их в релиз и вернёт ошибку:

```text
invalid ownership metadata
```

Перед первой реальной Helm-установкой нужно удалить конфликтующие ресурсы приложения, сохранив PostgreSQL, RabbitMQ, MinIO, PVC и Secret.

Удалить server и worker:

```bash
kubectl delete \
  -f k8s/base/server-deployment.yaml \
  -f k8s/base/server-service.yaml \
  -f k8s/base/worker-deployment.yaml \
  -f k8s/base/worker-service.yaml \
  --ignore-not-found
```

Удалить ConfigMap, Ingress, HPA, NetworkPolicy и RBAC:

```bash
kubectl delete \
  -f k8s/base/configmap.yaml \
  -f k8s/base/ingress.yaml \
  -f k8s/base/server-hpa.yaml \
  -f k8s/base/worker-hpa.yaml \
  -f k8s/base/networkpolicy-ingress.yaml \
  -f k8s/base/networkpolicy-egress.yaml \
  -f k8s/base/rbac.yaml \
  --ignore-not-found
```

Secret не удалять:

```text
gophprofile-secret
```

Dev-зависимости и PVC также не удалять.

Проверить оставшиеся ресурсы:

```bash
kubectl get all -n gophprofile
kubectl get secret,pvc -n gophprofile
```

PodDisruptionBudget из `k8s/base` в текущей версии не создаётся Helm Chart-ом. Если PDB уже применены, они могут оставаться отдельными kubectl-managed ресурсами и продолжат выбирать Helm Pod-ы по совместимым labels.

## Первая локальная установка

Установить релиз:

```bash
helm upgrade --install gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --wait \
  --timeout 10m
```

Команда `upgrade --install` означает:

* установить релиз, если его ещё нет;
* обновить релиз, если он уже существует.

Проверить статус:

```bash
helm status gophprofile -n gophprofile
```

Список релизов:

```bash
helm list -n gophprofile
```

Проверить ресурсы релиза:

```bash
kubectl get pods,svc,ingress,hpa \
  -n gophprofile \
  -l app.kubernetes.io/instance=gophprofile
```

Проверить применённые values:

```bash
helm get values gophprofile -n gophprofile
```

Показать все итоговые values:

```bash
helm get values gophprofile \
  -n gophprofile \
  --all
```

Показать сгенерированные манифесты:

```bash
helm get manifest gophprofile -n gophprofile
```

Показать Helm hooks:

```bash
helm get hooks gophprofile -n gophprofile
```

Показать NOTES повторно:

```bash
helm get notes gophprofile -n gophprofile
```

## Migration hook

Migration Job запускается как:

```text
pre-install
pre-upgrade
```

Это означает:

* миграции выполняются перед первой установкой приложения;
* миграции выполняются перед каждым обновлением релиза.

Проверить hook:

```bash
helm get hooks gophprofile -n gophprofile
```

Успешный migration Job удаляется согласно:

```text
before-hook-creation,hook-succeeded
```

Если миграции завершились ошибкой, Job остаётся для диагностики:

```bash
kubectl get jobs -n gophprofile
kubectl logs job/gophprofile-migrations -n gophprofile
kubectl describe job gophprofile-migrations -n gophprofile
```

## Проверка после установки

Проверить Pod-ы:

```bash
kubectl get pods -n gophprofile
```

Посмотреть server logs:

```bash
kubectl logs \
  -l app.kubernetes.io/instance=gophprofile,app.kubernetes.io/component=server \
  -n gophprofile \
  --tail=100
```

Посмотреть worker logs:

```bash
kubectl logs \
  -l app.kubernetes.io/instance=gophprofile,app.kubernetes.io/component=worker \
  -n gophprofile \
  --tail=100
```

Проверить через Ingress:

```bash
curl -i http://gophprofile.local/live
curl -i http://gophprofile.local/ready
curl -i http://gophprofile.local/health
curl -i http://gophprofile.local/metrics
```

Если Ingress недоступен, использовать port-forward:

```bash
kubectl port-forward \
  svc/gophprofile-server \
  8080:80 \
  -n gophprofile
```

В другом терминале:

```bash
curl -i http://localhost:8080/live
curl -i http://localhost:8080/ready
curl -i http://localhost:8080/health
curl -i http://localhost:8080/metrics
```

## Обновление релиза

После изменения templates или values выполнить:

```bash
helm upgrade gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --wait \
  --timeout 10m
```

Проверить:

```bash
helm status gophprofile -n gophprofile
helm history gophprofile -n gophprofile
```

### Обновление локального image с тем же тегом

При локальной разработке используется тег:

```text
local
```

Если image был пересобран с тем же тегом, Deployment-манифест не изменится, поэтому Helm может не перезапустить Pod.

После пересборки:

```bash
docker build \
  -f docker/server.Dockerfile \
  -t docker.io/library/gophprofile-server:local .
```

можно выполнить:

```bash
kubectl rollout restart \
  deployment/gophprofile-server \
  -n gophprofile
```

Для worker:

```bash
kubectl rollout restart \
  deployment/gophprofile-worker \
  -n gophprofile
```

Для production следует использовать неизменяемые version tags:

```text
1.0.0
1.0.1
commit-sha
```

и изменять `image.tag` при каждом релизе.

## Обновление через параметры

Пример изменения количества server-реплик:

```bash
helm upgrade gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --set server.replicaCount=2
```

Проверить:

```bash
kubectl get deployment gophprofile-server \
  -n gophprofile
```

Следует помнить: параметры `--set` сохраняются в Helm release. Для воспроизводимости постоянные настройки лучше записывать в values-файл.

## Включение Ingress

Локальные values уже включают Traefik Ingress:

```yaml
ingress:
  enabled: true
  className: traefik
  host: gophprofile.local
```

Проверить:

```bash
kubectl get ingress -n gophprofile
```

Для локального host добавить в `/etc/hosts`:

```bash
grep -q "gophprofile.local" /etc/hosts || \
  echo "127.0.0.1 gophprofile.local" | sudo tee -a /etc/hosts
```

## Включение HPA

Через командную строку:

```bash
helm upgrade gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --set autoscaling.server.enabled=true \
  --set autoscaling.worker.enabled=true
```

Проверка:

```bash
kubectl get hpa -n gophprofile
kubectl top pods -n gophprofile
```

При включённом HPA поле `spec.replicas` в Deployment не генерируется.

Для работы HPA нужен Metrics Server.

## Включение ServiceMonitor

ServiceMonitor требует Prometheus Operator и CRD:

```text
servicemonitors.monitoring.coreos.com
```

Проверить:

```bash
kubectl get crd servicemonitors.monitoring.coreos.com
```

Если CRD существует:

```bash
helm upgrade gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --set serviceMonitor.enabled=true
```

Если CRD отсутствует, ServiceMonitor включать нельзя.

## Включение NetworkPolicy

Перед включением убедиться, что CNI поддерживает NetworkPolicy и правила соответствуют реальной сети.

Включить:

```bash
helm upgrade gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --set networkPolicy.enabled=true
```

Проверить:

```bash
kubectl get networkpolicy -n gophprofile
kubectl describe networkpolicy -n gophprofile
```

После включения обязательно проверить:

* DNS;
* PostgreSQL;
* RabbitMQ;
* MinIO;
* Ingress;
* metrics;
* пользовательские запросы.

## Production example

Проверить production example:

```bash
helm lint charts/gophprofile \
  -f charts/gophprofile/values.prod.example.yaml
```

Рендеринг:

```bash
helm template gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.prod.example.yaml \
  > /tmp/gophprofile-prod.yaml
```

В production example необходимо заменить:

* адрес container registry;
* image tags;
* Ingress host;
* TLS Secret;
* S3 endpoint и bucket;
* OpenTelemetry endpoint;
* дополнительные labels Prometheus Operator;
* стратегию Secret;
* NetworkPolicy под реальную инфраструктуру.

Production example нельзя применять без предварительной адаптации.

## История релизов

Посмотреть историю:

```bash
helm history gophprofile -n gophprofile
```

Пример:

```text
REVISION  UPDATED                  STATUS      CHART
1         ...                      superseded  gophprofile-0.1.0
2         ...                      deployed    gophprofile-0.1.0
```

Каждый успешный `helm install`, `helm upgrade` или `helm rollback` создаёт новую revision.

## Rollback

Посмотреть историю:

```bash
helm history gophprofile -n gophprofile
```

Вернуться к конкретной revision:

```bash
helm rollback gophprofile 1 \
  -n gophprofile \
  --wait \
  --timeout 10m
```

Проверить:

```bash
helm status gophprofile -n gophprofile
helm history gophprofile -n gophprofile
kubectl get pods -n gophprofile
```

Важно: Helm rollback возвращает Kubernetes-манифесты предыдущей revision, но не выполняет автоматический rollback схемы PostgreSQL.

Если новая миграция несовместима со старой версией приложения, откат приложения может потребовать отдельного плана миграции БД.

## Atomic upgrade

Для более безопасного обновления можно использовать:

```bash
helm upgrade gophprofile charts/gophprofile \
  --namespace gophprofile \
  -f charts/gophprofile/values.local.yaml \
  --atomic \
  --wait \
  --timeout 10m
```

Если обновление не завершится успешно, Helm автоматически попытается вернуть предыдущую revision.

Это не отменяет уже выполненные необратимые миграции PostgreSQL.

## Удаление релиза

Удалить Helm release:

```bash
helm uninstall gophprofile -n gophprofile
```

Проверить:

```bash
helm list -n gophprofile
kubectl get all -n gophprofile
```

Helm удалит ресурсы, которыми управляет release.

Не будут автоматически удалены:

* namespace;
* существующий `gophprofile-secret`;
* PostgreSQL;
* RabbitMQ;
* MinIO;
* PVC и данные dev-зависимостей;
* ресурсы, созданные отдельно через `kubectl`.

Полностью удалить локальное окружение вместе с данными:

```bash
kubectl delete namespace gophprofile
```

## Диагностика

### Ошибка ownership metadata

Пример:

```text
resource already exists and cannot be imported into the current release
invalid ownership metadata
```

Причина: ресурс уже создан через `kubectl` и не принадлежит Helm release.

Решение: удалить конфликтующий kubectl-managed ресурс и повторить установку.

Не рекомендуется вручную добавлять Helm labels и annotations к большому набору существующих ресурсов без чёткого понимания последствий.

### Migration hook завершился ошибкой

Проверить:

```bash
kubectl get jobs -n gophprofile
kubectl logs job/gophprofile-migrations -n gophprofile
kubectl describe job gophprofile-migrations -n gophprofile
```

Проверить PostgreSQL и Secret:

```bash
kubectl get pods \
  -l app.kubernetes.io/component=postgres \
  -n gophprofile

kubectl get secret gophprofile-secret -n gophprofile
```

### `ErrImageNeverPull`

Проверить local images:

```bash
docker images | grep gophprofile
```

Пересобрать отсутствующий image.

### ServiceMonitor CRD отсутствует

Ошибка:

```text
no matches for kind "ServiceMonitor"
```

Отключить:

```yaml
serviceMonitor:
  enabled: false
```

или установить Prometheus Operator.

### Релиз завис в состоянии pending

Проверить:

```bash
helm status gophprofile -n gophprofile
helm history gophprofile -n gophprofile
kubectl get events -n gophprofile \
  --sort-by=.lastTimestamp | tail -50
```

При необходимости откатить или удалить неуспешный релиз.

## Полезные команды

```bash
helm list -n gophprofile
helm status gophprofile -n gophprofile
helm history gophprofile -n gophprofile
helm get values gophprofile -n gophprofile --all
helm get manifest gophprofile -n gophprofile
helm get hooks gophprofile -n gophprofile
helm get notes gophprofile -n gophprofile
```
