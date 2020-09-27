# Purpose

Discover services in docker and update front proxy's(based on envoy) configuration

# Building and Running it

```
  go build 
  ./whale_disco 
```
** pair it with an [envoy front-proxy](sample/front-proxy) 

# Notes

- EDS config dump is not available by default in the admin site, specify `include_eds` query parameter to make it visible, e.g `http://localhost:9901/config_dump?include_eds`

