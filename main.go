package main

import (
  "crypto/tls"
  "github.com/brianhempel/sneakynote.com/store"
  "log"
  "net/http"
  "sync/atomic"
  "time"
  "os"
  "os/signal"
  "syscall"
)

var (
  mainStore *store.Store
  lastStatusLogTime time.Time
)

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
  StartPeriodicStatusLogger()

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
    go http.ListenAndServe(":80", RedirectToHTTPSHandler())
    log.Print("Using TLS")
    server := &http.Server{
      Addr:      ":" + port,
      Handler:   AddHSTSHeader(Handlers()),
      TLSConfig: TLSConfig(),
    }
    err := server.ListenAndServeTLS(certs, privateKey)
    if err != nil {
      log.Fatal("ListenAndServeTLS: ", err)
    }
  }
}

func TLSConfig() *tls.Config {
  return &tls.Config{
    MinVersion: tls.VersionTLS10,
    PreferServerCipherSuites: true,
    CipherSuites: []uint16{
      tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
      tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
      tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
      tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
      tls.TLS_RSA_WITH_AES_256_CBC_SHA,
      tls.TLS_RSA_WITH_AES_128_CBC_SHA,
    },
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

func StartPeriodicStatusLogger() {
  lastStatusLogTime = time.Now()

  ticker := time.NewTicker(time.Hour * 3)
  go func() {
    for range ticker.C {
      logStatus()
    }
  }()

  signalChan := make(chan os.Signal, 1)
  signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM)
  go func() {
    <-signalChan
    logStatus()
    os.Exit(0)
  }()
}

func logStatus() {
  now := time.Now()

  created := atomic.SwapUint64(&notesCreatedCount, 0)
  full := atomic.SwapUint64(&noteStorageFullRequestCount, 0)
  tooLarge := atomic.SwapUint64(&noteTooLargeRequestCount, 0)
  duplicateId := atomic.SwapUint64(&noteDuplicateIdRequestCount, 0)
  opened := atomic.SwapUint64(&notesOpenedCount, 0)
  expired := atomic.SwapUint64(&noteExpiredRequestCount, 0)
  alreadyOpened := atomic.SwapUint64(&noteAlreadyOpenedRequestCount, 0)
  notFound := atomic.SwapUint64(&noteNotFoundCount, 0)
  status := atomic.SwapUint64(&statusRequestCount, 0)
  assets := atomic.SwapUint64(&assetRequestCount, 0)
  total := atomic.SwapUint64(&totalRequestCount, 0)

  requestsPerSecond := float64(total) / now.Sub(lastStatusLogTime).Seconds()

  log.Printf("Requests: total=%d rps=%.6f assets=%d Notes: created=%d opened=%d alreadyOpened=%d expired=%d notFound=%d full=%d tooLarge=%d duplicateId=%d status=%d",
    total,
    requestsPerSecond,
    assets,
    created,
    opened,
    alreadyOpened,
    expired,
    notFound,
    full,
    tooLarge,
    duplicateId,
    status)

  lastStatusLogTime = now
}