{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "jfrog-registry-operator.name" -}}
{{- include "common.names.name" . -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "jfrog-registry-operator.fullname" -}}
{{- include "common.names.fullname" . -}}
{{- end -}}

{{/*
Return the proper jfrog-registry-operator image name
*/}}
{{- define "jfrog-registry-operator.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.image "global" .Values.global "appVer" .Chart.AppVersion) }}
{{- end -}}

{{/*
Return the proper image name (for the init container volume-permissions image)
*/}}
{{- define "jfrog-registry-operator.initContainers.image" -}}
{{ include "common.images.image" (dict "imageRoot" .Values.initContainers.image "global" .Values.global) }}
{{- end -}}

{{/*
Return the proper Docker Image Registry Secret Names
*/}}
{{- define "jfrog-registry-operator.imagePullSecrets" -}}
{{ include "common.images.pullSecrets" (dict "images" (list .Values.image .Values.initContainers.image) "global" .Values.global) }}
{{- end -}}

{{/*
Return podAnnotations
*/}}
{{- define "jfrog-registry-operator.podAnnotations" -}}
{{- if .Values.podAnnotations }}
{{ include "common.tplvalues.render" (dict "value" .Values.podAnnotations "context" $) }}
{{- end }}
{{- if and .Values.metrics.enabled .Values.metrics.podAnnotations }}
{{ include "common.tplvalues.render" (dict "value" .Values.metrics.podAnnotations "context" $) }}
{{- end }}
{{- end -}}

{{/*
 Create the name of the service account to use
 */}}
{{- define "jfrog-registry-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "jfrog-registry-operator.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Return podAnnotations
*/}}
{{- define "jfrog-admission-controller.podAnnotations" -}}
{{- if .Values.podAnnotations }}
{{ include "common.tplvalues.render" (dict "value" .Values.podAnnotations "context" $) }}
{{- end }}
{{- if and .Values.metrics.enabled .Values.metrics.podAnnotations }}
{{ include "common.tplvalues.render" (dict "value" .Values.metrics.podAnnotations "context" $) }}
{{- end }}
{{- end -}}
