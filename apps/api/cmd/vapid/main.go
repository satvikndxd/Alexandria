// Command vapid generates a VAPID keypair for Web Push.
//
//	go run ./cmd/vapid
//	PUSH_VAPID_PUBLIC=… PUSH_VAPID_PRIVATE=… go run ./cmd/api
//
// The private half is a server secret: rotate it and every existing
// subscription must be re-created by the reader's browser, which is the
// correct consequence of losing a signing key.
package main

import (
	"fmt"
	"log"

	"github.com/alexandria-reads/alexandria/apps/api/internal/push"
)

func main() {
	pub, priv, err := push.GenerateVAPID()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("PUSH_VAPID_PUBLIC=" + pub)
	fmt.Println("PUSH_VAPID_PRIVATE=" + priv)
	fmt.Println("PUSH_SUBJECT=mailto:you@yourdomain.example")
}
