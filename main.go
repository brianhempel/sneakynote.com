package main

import (
  "github.com/brianhempel/sneakynote.com/store"
  "log"
  "net/http"
  "os"
)

var mainStore *store.Store

func main() {
  if len(os.Args) == 1 {
    StartServer()
  } else if os.Args[1] == "setup" {
    SetupStore()
  } else if os.Args[1] == "teardown" {
    TeardownStore()
  } else {
    log.Print("Invalid argument ", os.Args[1])
    log.Print("  ")
    log.Print("No arguments starts the server.")
    log.Print("  ")
    log.Print("./sneakynote.com setup")
    log.Print("will set up the datastore.")
    log.Print("  ")
    log.Print("./sneakynote.com teardown")
    log.Print("will tear down the datastore.")
    os.Exit(1)
  }
}

func StartServer() {
  MaybeSetupStore()

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

func GetStore() {
  mainStore = store.Get()
}

func MaybeSetupStore() {
  if _, err := os.Stat(store.Get().ExpiredPath); os.IsNotExist(err) {
    SetupStore()
  } else {
    GetStore()
  }
}

func SetupStore() {
  log.Printf("Setting up datastore...")
  mainStore = store.Setup()
}

func TeardownStore() {
  log.Printf("Tearing down datastore...")
  if mainStore == nil {
    GetStore()
  }
  mainStore.Teardown()
}

func StartSweeper() {
  go mainStore.SweepContinuously()
}
