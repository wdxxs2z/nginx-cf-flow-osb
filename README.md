# Nginx flow example service broker

Platform support: only cloudfoundry (k8s has integrate with istio, so balabala...)</br>

### Create service instance like, this use cf api and create nginx application as flow gateway

```
{
  "space_guid": "bbbeed31-f908-477a-aab9-8cdcd19e1348",
  "organization_guid": "bbbeed31-f908-477a-aab9-8cdcd19e13dd",
  "service_id": "7eab5451-8200-4c65-982a-0f04b5a3ef6f",
  "plan_id": "7f6ac61e-f449-4a70-8309-875c6250c1c1",
  "name": "my-service-instance",
  "service_plan_guid": "2109449e-f6b9-4002-b4ec-3c81c582c072",
  "parameters": {
    "host": "fake",
    "domain": "local.pcfdev.io",
    "nginxs": "[{\"name\":\"fakea\",\"url\":\"fakea.local.pcfdev.io\",\"weight\":4,\"port\":8001},{\"name\":\"fakeb\",\"url\":\"fakeb.local.pcfdev.io\",\"weight\":6,\"port\":8002}]"
  }
}
```