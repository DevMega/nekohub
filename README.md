## Installation Guide
### Dependencies
* Install golang
* Install docker
* Install nginx
#### golang
For installing golang follow inststruction at <https://golang.org/doc/install>
#### Docker
For installing docker follow inststruction at <https://docs.docker.com/engine/install/>

Pull docker image

```docker pull nurdism/neko:firefox```

#### nginx setup
For installing nginx follow instruction at 
<https://www.nginx.com/resources/wiki/start/topics/tutorials/install/>

Remove ``default`` from ``/etc/nginx/sites-enabled``

```sudo rm /etc/nginx/sites-enabled/default```

Create file at ``/etc/nginx/conf.d`` and with filename ``domain.conf`` with contains and replate [YOUR-DOMAIN] to your domain

```
server {
    listen 127.0.0.1:80;
    server_name [YOUR-DOAMIN]; #example.com www.example.com

    location / {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_pass         http://127.0.0.1:5000;
    }
}

server {
    listen 127.0.0.1:80;
    server_name "~^rooms-(\d{4}).[YOUR-DOMAIN]";

    location / {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_pass         http://127.0.0.1:$1;
    }
}


```

### Compileing Project
go project folder and compile project with
EDIT YOUR DOMAIN NAME IN ``templates/dashboard.html line no:33 localhost to [YOUR-DOMAIN]``

```go build main.go```

Edit nekohub.service and
Copy nekohub.service to the ``/etc/systemd/system/``

Start our server with

```systemctl start nekohub```

All done! test it