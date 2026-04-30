package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"orquestador_p/internal/webserver"
)

func main() {
	port := flag.String("port", "8080", "Puerto del servidor web")
	flag.Parse()

	srv := webserver.New(runOrchestrator, staticFiles)
	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Orquestador iniciado → http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, srv))
}
