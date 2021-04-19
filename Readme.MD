# Purpose

Discover services in docker and update front proxy's(based on envoy) configuration

# Discoverable Containers

Only containers that have `CLUSTER_<port>_NAME` label are discoverable, e.g. 

```
    LABEL CLUSTER_80_NAME=serviceA
```

The default front proxy path will take after the name, base on the example above this would be `/serviceA`.

This can be overridden by adding a prefix label, e.g.

```
    LABEL CLUSTER_80_URLPREFIX="/api/service1"  
```

# Building and Running it

```
  go build 
  ./whale_disco 
```
** pair it with an [envoy front-proxy](sample/front-proxy)  
** for a slightly less mickey mousy [sample](https://github.com/kahgeh/devenv/blob/master/aws/front-proxy/envoy-template.py#L5)

# Set it up as a service through aws ec2 userdata 


```
    cat <<'EOF' >> /lib/systemd/system/whale-disco.service
    [Unit]
    Description=Whale Disco - Docker Based Discovery
    After=network.target
    StartLimitIntervalSec=0
    [Service]
    Type=simple
    Restart=always
    RestartSec=1
    User=root
    StandardOutput=file:/var/log/whale-disco/logs.log
    StandardError=file:/var/log/whale-disco/logs.log
    ExecStart=/usr/bin/whale-disco -domain=${DomainName}
    
    [Install]
    WantedBy=multi-user.target
    EOF

    systemctl start whale-disco.service
```

# Manual testing with samples

## Steps
1. Run the [services](sample/services/Readme.MD) 
2. Build and run `whale-disco`
3. Run the [front-proxy](sample/front-proxy/Readme.MD)
 
## Observations 
1. service1 uses an `/api/service1` url prefix label, so can be accessed here `http://localhost:10000/api/service1`
   refresh the page, notice that it is alternately being served by two containers
2. service2 does not have a url prefix label, so it can be accessed here `http://localhost:10000/service2/test`

                
# Notes

- EDS config dump is not available by default in the admin site, specify `include_eds` query parameter to make it visible, e.g `http://localhost:9901/config_dump?include_eds`



