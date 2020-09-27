# Build

    docker build . -t service1
 
# Run 

    docker run --rm --name container1 -e CONTAINER_NAME=container1 -d service1
