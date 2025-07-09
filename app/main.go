package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Helfjfjfejawghey!")
	})

	log.Println(" ttph://a8ffgegwegwgwrwrgrwrwrrgfn n0d8d0")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Fatal error: %s", err)
	}
}