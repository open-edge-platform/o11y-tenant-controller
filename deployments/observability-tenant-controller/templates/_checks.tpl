{{ define "checks" }}
  {{- $verifyModes := list "loose" "strict" }}
  {{ if not (mustHas .Values.loki.deleteVerifyMode $verifyModes) }}
  {{ fail "please provide correct .Values.loki.deleteVerifyMode value" }}
  {{ end }}
  {{ if not (mustHas .Values.mimir.deleteVerifyMode $verifyModes) }}
  {{ fail "please provide correct .Values.mimir.deleteVerifyMode value" }}
  {{ end }}
{{ end }}
