# Run docker 

    docker run --name envoy --rm -it -p 10000:10000 -p 9901:9901 -v $PWD/envoy.yaml:/etc/envoy/envoy.yaml envoyproxy/envoy-alpine:v1.15-latest 
