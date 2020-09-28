package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, request *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		fmt.Fprintf(w, "{ \"hello from\": \"%s\", \"path\": %q }\n", os.Getenv("CONTAINER_NAME"), request.URL)
	})
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
}
