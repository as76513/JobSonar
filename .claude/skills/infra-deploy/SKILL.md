---
name: infra-deploy
description: Modify JobRadar infrastructure or deployment (Terraform, Helm/k8s manifests, CronJobs, External Secrets, IRSA). Use for cloud or local cluster changes. Do NOT use for application logic.
---
# Infra & deploy
1. Every change must keep the local (kind/k3s + Ollama, no cloud) path working.
2. No secrets in manifests: source from Secrets Manager via External Secrets Operator.
3. Each pod gets least-privilege AWS access via IRSA — no node-wide/long-lived keys.
4. Same container image runs local and cloud; only config/manifests differ.
5. New services wire the OpenTelemetry exporter and a ServiceMonitor.
6. Keep AWS SDK calls behind the adapter layer.
