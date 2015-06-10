package main

import (
  "github.com/brianhempel/sneakynote.com/store"
  "log"
  "net/http"
  "os"
)

var mainStore *store.Store

func main() {
  log.Printf("Setting up datastore...")
  SetupStore()
  defer TeardownStore()

  log.Printf("Starting sweeper...")
  StartSweeper()

  port := os.Getenv("SNEAKYNOTE_PORT")
  certs := os.Getenv("SNEAKYNOTE_CERTS")
  privateKey := os.Getenv("SNEAKYNOTE_PRIVATE_KEY")

  if port == "" {
    port = "8080"
  }

  log.Printf("Starting SneakyNote server on port " + port + "!")

  if certs == "" || privateKey == "" {
    err := http.ListenAndServe(":" + port, Handlers())
    if err != nil {
      log.Fatal("ListenAndServe: ", err)
    }
  } else {
    log.Print("Using TLS")
    err := http.ListenAndServeTLS(":" + port, certs, privateKey, Handlers())
    if err != nil {
      log.Fatal("ListenAndServeTLS: ", err)
    }
  }
}

func SetupStore() {
  mainStore = store.Setup()
}

func TeardownStore() {
  mainStore.Teardown()
}

func StartSweeper() {
  go mainStore.SweepContinuously()
}
