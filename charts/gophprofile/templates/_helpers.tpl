{{/*
Возвращает базовое имя chart-а.
*/}}
{{- define "gophprofile.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Возвращает полное имя релиза.
*/}}
{{- define "gophprofile.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Возвращает namespace для ресурсов.
*/}}
{{- define "gophprofile.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride -}}
{{- end -}}

{{/*
Общие labels для всех ресурсов chart-а.
*/}}
{{- define "gophprofile.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ include "gophprofile.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/part-of: gophprofile
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels. Эти labels должны совпадать между Deployment и Service.
*/}}
{{- define "gophprofile.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gophprofile.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Имя ServiceAccount приложения.
*/}}
{{- define "gophprofile.serviceAccountName" -}}
{{- default (printf "%s-app" (include "gophprofile.fullname" .)) .Values.rbac.serviceAccountName -}}
{{- end -}}

{{/*
Возвращает имя ConfigMap приложения.
*/}}
{{- define "gophprofile.configMapName" -}}
{{- printf "%s-config" (include "gophprofile.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Возвращает имя Secret приложения.
Secret может быть создан Helm Chart-ом или существовать заранее.
*/}}
{{- define "gophprofile.secretName" -}}
{{- default (printf "%s-secret" (include "gophprofile.fullname" .)) .Values.secret.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Возвращает имя Kubernetes-ресурсов server-а.
*/}}
{{- define "gophprofile.serverName" -}}
{{- printf "%s-server" (include "gophprofile.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Возвращает имя Kubernetes-ресурсов worker-а.
*/}}
{{- define "gophprofile.workerName" -}}
{{- printf "%s-worker" (include "gophprofile.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}