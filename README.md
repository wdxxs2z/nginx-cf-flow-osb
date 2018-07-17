# Nginx flow example service broker

Platform support: only cloudfoundry (k8s has integrate with istio, so balabala...)</br>

When the nginx proxy application update, we will start a new blue application,and wait the application state become running,then delete origin app.</br>

### Create nginx service instance with two parameters

**host:** the nginx global host name </br>
**domain:** the application shared domain </br>

**curl:** http://127.0.0.1:8080/v2/service_instances/${service_instance_guid} </br>

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
    "domain": "local.pcfdev.io"
  }
}
```

### bind a application to the nginx proxy service instance

**url:** assign the bind application url to nginx, if not set, assign the application default first route </br>
**weight:** assign the url weight to proxy nginx service instance </br>

**curl:** http://127.0.0.1:8080/v2/service_instances/${service_instance_guid}/service_bindings/${service_binding_guid}

```
{
  "space_guid": "bbbeed31-f908-477a-aab9-8cdcd19e1348",
  "app_guid": "fb7e12e9-6549-404c-9885-23b5b6df17c7",
  "organization_guid": "bbbeed31-f908-477a-aab9-8cdcd19e13dd",
  "service_id": "7eab5451-8200-4c65-982a-0f04b5a3ef6f",
  "plan_id": "7f6ac61e-f449-4a70-8309-875c6250c1c1",
  "name": "my-service-instance",
  "service_plan_guid": "2109449e-f6b9-4002-b4ec-3c81c582c072",
  "parameters": {
    "weight": 4
  }
}
```

### unbind the application url

**curl:** http://127.0.0.1:8080/v2/service_instances/${service_instance_guid}/service_bindings/${service_binding_guid}?service_id=7eab5451-8200-4c65-982a-0f04b5a3ef6f&plan_id=7f6ac61e-f449-4a70-8309-875c6250c1c1 </br>

### the nginx proxy template

```
server {
    listen 8002;
    server_name 7b2e7d2c915242a5befcf03e1c3f47cy;
    location / {
        proxy_pass       http://fakea.local.pcfdev.io;
        proxy_set_header Host fakea.local.pcfdev.io;
    }
}

upstream 00027581-474a-4894-b353-0888b4df26ec {
    server 127.0.0.1:8002  weight=4;
    keepalive 20000;
}

server {
    listen 8080;
    server_name localhost;

    location / {
      proxy_redirect off;
      proxy_pass http://00027581-474a-4894-b353-0888b4df26ec;
    }

    location ~ /\. {
      deny all;
      return 404;
    }
}
```