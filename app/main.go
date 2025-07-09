package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Helfjfjfwghey!")
	})

	log.Println(" ttph://a8ffg    lnn j  j  j gwegwgwrwrgrwrwrrgfn n0d8d0")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Fatal error: %s", err)
	}
}