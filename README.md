# knatify

Knatify is a tool to detect if existing Kubernetes deployments can potentially migrated into a Knative Serving application as-is.

## Ideas

- [ ] Detect if a deployment can be migrated to a Knative Service
- [ ] Detect if an already deployed deployment can be migrated to a Knative Service
- [ ] Mark all running deployments that can be migrated with an annotation
- [ ] Write a corresponding operator to migrate all of the marked deployments