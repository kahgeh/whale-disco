# Build

    docker build . -f Dockerfile.1 -t service1
    docker build . -f Dockerfile.2 -t service2
 
# Run 

    docker run --rm --name container11 -e CONTAINER_NAME=container11 -d service1
    docker run --rm --name container12 -e CONTAINER_NAME=container12 -d service1
    docker run --rm --name container21 -e CONTAINER_NAME=container21 -d service2
    docker run --rm --name container22 -e CONTAINER_NAME=container22 -d service2
    
