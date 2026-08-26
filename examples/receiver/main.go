package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/signing"
)

func main() {
	listenAddress := flag.String("listen", "127.0.0.1:3000", "loopback listen address")
	publicKeyPath := flag.String("public-key", ".kick-sim/keys/public-key.pem", "simulator public key")
	flag.Parse()

	publicKey, err := signing.ReadPublicKey(*publicKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhooks/kick", func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
		if err != nil {
			http.Error(writer, "read body", http.StatusBadRequest)
			return
		}
		if err := signing.Verify(
			publicKey,
			request.Header.Get(delivery.HeaderMessageID),
			request.Header.Get(delivery.HeaderTimestamp),
			body,
			request.Header.Get(delivery.HeaderSignature),
		); err != nil {
			http.Error(writer, "invalid signature", http.StatusUnauthorized)
			return
		}

		fmt.Printf("verified %s@%s (%s)\n",
			request.Header.Get(delivery.HeaderEventType),
			request.Header.Get(delivery.HeaderEventVersion),
			request.Header.Get(delivery.HeaderMessageID),
		)
		writer.WriteHeader(http.StatusNoContent)
	})

	log.Printf("verifying receiver listening on http://%s/webhooks/kick", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, mux))
}
