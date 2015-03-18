package main

import (
  "github.com/brianhempel/sneakynote.com/store"
  "log"
  "net/http"
)

var mainStore *store.Store

func main() {
  log.Printf("Setting up datastore...")
  SetupStore()
  defer TeardownStore()

  log.Printf("Starting sweeper...")
  StartSweeper()

  log.Printf("Starting SneakyNote server on port 8080!")
  err := http.ListenAndServe(":8080", Handlers())
  if err != nil {
    log.Fatal("ListenAndServe: ", err)
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
