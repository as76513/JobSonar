package score

// Lexicon is the same token set the Python resume parser uses
// (services/agent/jobsonar_agent/resume/lexicon.py). Job-side extract
// lives here so Go never calls the agent over HTTP.
var Lexicon = []string{
	"kubernetes", "k8s", "terraform", "aws", "azure", "gcp",
	"docker", "devops", "devsecops", "ci/cd", "cicd", "linux",
	"python", "go", "golang", "java", "rust", "typescript", "javascript",
	"node", "react", "helm", "ansible", "puppet", "chef",
	"prometheus", "grafana", "datadog", "splunk", "elk", "elasticsearch",
	"kafka", "redis", "postgres", "postgresql", "mysql", "mongodb", "sql",
	"git", "github", "github actions", "gitlab", "jenkins", "circleci",
	"argocd", "argo cd", "flux", "istio", "linkerd", "nginx", "envoy",
	"oauth", "oidc", "saml", "iam", "rbac", "sre", "platform",
	"observability", "opentelemetry", "otel", "security", "vault",
	"secrets manager", "irsa", "eks", "ecs", "lambda", "s3", "sqs", "rds",
	"cloudformation", "pulumi", "bash", "shell", "networking", "tcp",
	"dns", "tls", "incident", "oncall", "on-call",
}
