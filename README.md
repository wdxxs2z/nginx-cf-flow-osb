# Nginx flow example service broker

Platform support: only cloudfoundry (k8s has integrate with istio, so balabala...)</br>

When the nginx proxy application update, we will start a new blue application,and wait the application state become running,then delete origin app.</br>

### Start nginx service localhost

ginx-flow-osb -config nginx-flow-osb.yaml

**service_instance_space:** must create org: system and space: nginx-flow-osb

| Yaml arameter          | Description                            | Default                   |
| ----------------------- | -------------------------------------- | ------------------------- |
| `cf_api_url`|The cloud foundry api url|"https://api.local.pcfdev.io"|
| `cf_username`|The cloud foundry api username|"admin"|
| `cf_passwrod`|The cloud foundry api password |"admin"|
| `service_config.db`|Mysql database configuration|"db"|
| `store_data_dir`|The nginx service instance store data dir|""|
| `template_dir`|The nginx static template store data dir|""|
| `service_space`|Under the system org, default nginx service space instance|"nginx-flow-osb"|
| `plan.use_system_space`|The plan open system space service instance|true/false|

### Service broker environment
| ENV NAME          | Description                            |
| ----------------------- | -------------------------------------- |
| `CF_API`|Cloud foundry api url|
| `CF_USERNAME`|Cloud foundry user|
| `CF_PASSWORD`|Cloud foundry password|
| `DATABASE_NAME`|Mysql database name|
| `DATABASE_USERNAME`|Mysql database username|
| `DATABASE_HOST`|Mysql database host|
| `DATABASE_PORT`|Mysql database port|
| `DATABASE_PASSWORD`|Mysql database user password|

### Register the service broker to cloudfoundry

cf create-service-broker nginx-osb admin changeme http://nginx-service-broker.local.pcfdev.io</br>
cf enable-service-access nginx-flow-osb

### Create nginx service instance with two parameters

**host:** the nginx global host name (requred)</br>
**domain:** the application shared domain (requred)</br>

```
cf create-service nginx-flow-osb free nginx-test -c '{"host": "fake", "domain": "local.pcfdev.io"}'
```

### bind a application to the nginx proxy service instance

**url:** assign the bind application url to nginx, if not set, assign the application default first route (option)</br>
**weight:** assign the url weight to proxy nginx service instance </br>

```
cf bind-service fakea nginx-test -c '{"url": "fakea.local.pcfdev.io", "weight": 4}'
cf bind-service fakeb nginx-test -c '{"url": "fakeb.local.pcfdev.io", "weight": 6}'
```

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