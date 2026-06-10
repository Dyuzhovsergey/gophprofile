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

