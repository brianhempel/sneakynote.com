package main

import (
  "bufio"
  "github.com/brianhempel/sneakynote.com/store"
  "fmt"
  "log"
  "mime"
  "net/http"
  "path"
  "path/filepath"
  "regexp"
  "reflect"
  "runtime"
  "strconv"
  "strings"
  "sync/atomic"
  "time"
  "unsafe" // For forcing a flush of the response bufios
)

var (
  notePathRegexp = regexp.MustCompile("\\A/notes/([0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12})/?\\z")
  noteStatusPathRegexp = regexp.MustCompile("\\A/notes/([0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12})/status/?\\z")

  notesCreatedCount uint64 = 0
  noteStorageFullRequestCount uint64 = 0
  noteTooLargeRequestCount uint64 = 0
  noteDuplicateIdRequestCount uint64 = 0
  notesOpenedCount uint64 = 0
  noteExpiredRequestCount uint64 = 0
  noteAlreadyOpenedRequestCount uint64 = 0
  noteNotFoundCount uint64 = 0
  statusRequestCount uint64 = 0
  assetRequestCount uint64 = 0
  totalRequestCount uint64 = 0
)

func Handlers() *http.ServeMux {
  mux := http.NewServeMux()

  publicDir := http.Dir(publicPath())
  mux.Handle("/", CountAnAssetRequest(Cache1Day(MaybeGzip(publicDir, http.FileServer(publicDir)))))

  mux.HandleFunc("/free_space", freeSpace)

  mux.HandleFunc("/notes/", note)

  return mux;
}

func CountAnAssetRequest(original http.Handler) http.Handler {
  return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
    atomic.AddUint64(&totalRequestCount, 1)
    atomic.AddUint64(&assetRequestCount, 1)
    original.ServeHTTP(response, request)
  })
}

func Cache1Day(original http.Handler) http.Handler {
  return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
    // 1 Day
    response.Header()["Cache-Control"] = []string{"private, max-age=86400"}
    original.ServeHTTP(response, request)
  })
}

// Looks for a file.gz file and serves that if present
// `original` is a file server
func MaybeGzip(root http.FileSystem, original http.Handler) http.Handler {
  return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
    if strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
      requestPath := request.URL.Path
      if !strings.HasPrefix(requestPath, "/") {
      	requestPath = "/" + requestPath
      }
      requestPath = path.Clean(requestPath)
      gzPath := requestPath + ".gz"
      if requestPath == "/" {
        gzPath = "/index.html.gz"
      }
      // log.Print(gzPath)
      file, err := root.Open(gzPath)
      if err == nil {
        file.Close()
        contentType := mime.TypeByExtension(filepath.Ext(requestPath))
        if contentType == "" {
          contentType = "text/html; charset=utf-8"
        }
        response.Header().Set("Content-Type", contentType)
        response.Header().Set("Content-Encoding", "gzip")
      	request.URL.Path = gzPath
      }
		}
    original.ServeHTTP(response, request)
  })
}

func AddHSTSHeader(original http.Handler) http.Handler {
  return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
    // 2 Years
    response.Header()["Strict-Transport-Security"] = []string{"max-age=63072000; includeSubDomains; preload"}
    original.ServeHTTP(response, request)
  })
}

func RedirectToHTTPSHandler() http.Handler {
  return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
    newURL := "https://sneakynote.com" + request.URL.String()
    // log.Printf("Redirecting %s to %s", request.URL.String(), newURL)
    http.Redirect(response, request, newURL, http.StatusMovedPermanently)
  })
}

func note(response http.ResponseWriter, request *http.Request) {
  atomic.AddUint64(&totalRequestCount, 1)

  response.Header()["Cache-Control"] = []string{"private, max-age=0, no-cache, no-store"}

  if noteStatusPathRegexp.MatchString(request.URL.Path) {
    noteStatus(response, request)
    return
  }
  if !notePathRegexp.MatchString(request.URL.Path) {
    http.NotFoundHandler().ServeHTTP(response, request)
    return
  }

  switch request.Method {
  case "GET": getNote(response, request)
  case "POST": postNote(response, request)
  default: http.NotFoundHandler().ServeHTTP(response, request)
  }
}


func noteStatus(response http.ResponseWriter, request *http.Request) {
  switch request.Method {
  case "GET": getNoteStatus(response, request)
  default: http.NotFoundHandler().ServeHTTP(response, request)
  }
}

func postNote(response http.ResponseWriter, request *http.Request) {
  defer zeroRequestBuffer(request)

  if request.ContentLength > int64(mainStore.MaxSecretSize) {
    atomic.AddUint64(&noteTooLargeRequestCount, 1)
    respondSecretTooLarge(response)
    return
  }

  parts := notePathRegexp.FindStringSubmatch(request.URL.Path)

  id := parts[1]

  code, err := mainStore.Save(request.Body, id)

  if err == store.SecretTooLarge {
    atomic.AddUint64(&noteTooLargeRequestCount, 1)
    respondSecretTooLarge(response)
    return
  } else if err == store.DuplicateId {
    atomic.AddUint64(&noteDuplicateIdRequestCount, 1)
    respondDuplicateId(response)
    return
  } else if err == store.StorageFull {
    atomic.AddUint64(&noteStorageFullRequestCount, 1)
    respondStorageFull(response)
    return
  } else if err != nil {
    response.WriteHeader(http.StatusInternalServerError) // 500
    log.Print("Returning 500:", err)
    return
  }

  atomic.AddUint64(&notesCreatedCount, 1)
  response.Header().Set("X-Note-Code", code)
  response.WriteHeader(http.StatusCreated) // 201
}

func getNote(response http.ResponseWriter, request *http.Request) {
  parts := notePathRegexp.FindStringSubmatch(request.URL.Path)

  id := parts[1]

  buf := make([]byte, mainStore.MaxSecretSize)
  defer zeroBuffer(buf)

  nRead, code, err := mainStore.Retrieve(id, buf)

  if err == store.SecretAlreadyAccessed {
    atomic.AddUint64(&noteAlreadyOpenedRequestCount, 1)
    response.WriteHeader(http.StatusForbidden) // 403
    return
  } else if err == store.SecretExpired {
    atomic.AddUint64(&noteExpiredRequestCount, 1)
    response.WriteHeader(http.StatusGone) // 410
    return
  } else if err == store.SecretNotFound {
    atomic.AddUint64(&noteNotFoundCount, 1)
    response.WriteHeader(http.StatusNotFound) // 404
    return
  } else if err != nil {
    response.WriteHeader(http.StatusInternalServerError) // 500
    log.Print("Returning 500:", err)
    return
  }

  atomic.AddUint64(&notesOpenedCount, 1)
  response.Header().Set("Content-Type", "application/octet-stream")
  response.Header().Set("X-Note-Code", code)
  response.WriteHeader(http.StatusOK) // 200
  response.Write(buf[:nRead])
  zeroResponseBuffer(response)
}

func getNoteStatus(response http.ResponseWriter, request *http.Request) {
  atomic.AddUint64(&statusRequestCount, 1)

  parts := noteStatusPathRegexp.FindStringSubmatch(request.URL.Path)

  id := parts[1]

  code := request.Header.Get("X-Note-Code")

  timeout := time.Second * 0
  if request.Header.Get("X-Long-Poll") == "true" {
    timeout = time.Second * 8
  }
  timeoutTime := time.Now().Add(timeout)

  for {
    err := mainStore.Status(id, code)

    if err == store.SecretAlreadyAccessed {
      response.WriteHeader(http.StatusForbidden) // 403
      return
    } else if err == store.SecretExpired {
      response.WriteHeader(http.StatusGone) // 410
      return
    } else if err == store.SecretNotFound {
      response.WriteHeader(http.StatusNotFound) // 404
      return
    } else if err != nil {
      response.WriteHeader(http.StatusInternalServerError) // 500
      log.Print("Returning 500:", err)
      return
    }

    if time.Now().After(timeoutTime) {
      break
    } else {
      time.Sleep(time.Millisecond * 300)
    }
  }

  response.WriteHeader(http.StatusOK) // 200
}

func freeSpace(response http.ResponseWriter, request *http.Request) {
  atomic.AddUint64(&totalRequestCount, 1)

  response.Header()["Cache-Control"] = []string{"private, max-age=0, no-cache, no-store"}

  switch request.Method {
  case "GET": response.Write([]byte(fmt.Sprintf("%.3f MB\n", float64(mainStore.AvailableMemory()) / 1024.0 / 1024.0)))
  default: http.NotFoundHandler().ServeHTTP(response, request)
  }
}

// Root director for static files.
func publicPath() string {
  return path.Join(projectPath(), "public")
}

// Root directory of the SneakyNote.com project
func projectPath() string {
  _, thisFilePath, _, _ := runtime.Caller(1)
  return path.Dir(thisFilePath);
}

func respondSecretTooLarge(response http.ResponseWriter) {
  response.Header().Set("Content-Type", "application/json")
  response.WriteHeader(http.StatusRequestEntityTooLarge) // 413
  response.Write([]byte("{\n  \"error_type\": \"secret_too_large\",\n  \"error_message\": \"Secret too large. Maximum allowed secret size is " + strconv.FormatInt(int64(mainStore.MaxSecretSize), 10) + " bytes.\"\n}\n"))
}

func respondDuplicateId(response http.ResponseWriter) {
  response.Header().Set("Content-Type", "application/json")
  response.WriteHeader(http.StatusForbidden) // 403
  response.Write([]byte("{\n  \"error_type\": \"duplicate_id\",\n  \"error_message\": \"A secret with that ID has already been created. If you are not an attacker trying to replace the secret, this indicates a bug in your program and a potentially insecure source of randomness. As a precaution/penalty, the secret has been destroyed (if it has not already expired or been accessed).\"\n}\n"))
}
func respondStorageFull(response http.ResponseWriter) {
  response.Header().Set("Content-Type", "application/json")
  response.WriteHeader(507) // 507 Insufficient Storage
  response.Write([]byte("{\n  \"error_type\": \"storage_full\",\n  \"error_message\": \"Sorry, server secret storage is full right now. Try again later.\"\n}\n"))
}

// Wow, this works.
func zeroRequestBuffer(request *http.Request) {
  // log.Printf("%#v", request)
  // bodyType := reflect.TypeOf(request.Body)
  // log.Printf("type name %s", bodyType.Name())
  // log.Printf("type string %s", bodyType.String())
  // log.Printf("type kind %s", bodyType.Kind().String())

  bodyPtr := reflect.ValueOf(request.Body)
  // log.Printf("bodyPtr value %#v", bodyPtr)
  // log.Printf("bodyPtr value type %s", bodyPtr.Type().String())
  // log.Printf("bodyPtr value kind %s", bodyPtr.Kind().String())

  bodyValue := bodyPtr.Elem()
  // log.Printf("bodyValue value %#v", bodyValue)
  // log.Printf("bodyValue value type %s", bodyValue.Type().String())
  // log.Printf("bodyValue value kind %s", bodyValue.Kind().String())

  // If the client asked for a 100 response then the reader has one extra
  // wrapper we have to dive through.
  if (bodyValue.Type().String() == "http.expectContinueReader") {
    bodyPtr := bodyValue.FieldByName("readCloser").Elem()
    // log.Printf("bodyPtr value %#v", bodyPtr)
    // log.Printf("bodyPtr value type %s", bodyPtr.Type().String())
    // log.Printf("bodyPtr value kind %s", bodyPtr.Kind().String())
    bodyValue = bodyPtr.Elem()
  }

  bodySrcInt := bodyValue.FieldByName("src")
  // typeOfBodyDeref := bodyDeref.Type()
  // for i := 0; i < bodyDeref.NumField(); i++ {
  //     f := bodyDeref.Field(i)
  //     if typeOfBodyDeref.Field(i).Name == "src" {
  //       bodySrc = f
  //       log.Printf("%d: %s %s\n", i, typeOfBodyDeref.Field(i).Name, f.Type())
  //     }
  // }
  // log.Printf("bodySrc %#v", bodySrc)
  // log.Printf("bodySrc type %s", bodySrc.Type().String())
  // log.Printf("bodySrc kind %s", bodySrc.Kind().String())

  bodySrcLimitedReaderPtr := bodySrcInt.Elem()
  // log.Printf("bodySrc value %#v", bodySrcValue)
  // log.Printf("bodySrc value type %s", bodySrcValue.Type().String())
  // log.Printf("bodySrc value kind %s", bodySrcValue.Kind().String())

  bodySrcLimitedReader := bodySrcLimitedReaderPtr.Elem()
  // log.Printf("bodySrc value2 %#v", bodySrcValue2)
  // log.Printf("bodySrc value2 type %s", bodySrcValue2.Type().String())
  // log.Printf("bodySrc value2 kind %s", bodySrcValue2.Kind().String())

  bodyReader := bodySrcLimitedReader.FieldByName("R")
  // log.Printf("bodyReader %#v", bodyReader)
  // log.Printf("bodyReader type %s", bodyReader.Type().String())
  // log.Printf("bodyReader kind %s", bodyReader.Kind().String())

  bufioReaderPtr := bodyReader.Elem()
  // log.Printf("bufioReaderPtr %#v", bufioReaderPtr)
  // log.Printf("bufioReaderPtr type %s", bufioReaderPtr.Type().String())
  // log.Printf("bufioReaderPtr kind %s", bufioReaderPtr.Kind().String())

  bufioReader := bufioReaderPtr.Elem()
  // log.Printf("bufioReader %#v", bufioReader)
  // log.Printf("bufioReader type %s", bufioReader.Type().String())
  // log.Printf("bufioReader kind %s", bufioReader.Kind().String())

  // need to clear the bufioReader.buf []byte buffer

  bufioBuf := bufioReader.FieldByName("buf")

  bufioBufSlice := bufioBuf.Bytes()

  // log.Printf("slice before: %#v", bufioBufSlice)

  zeroBuffer(bufioBufSlice)

  // log.Printf("slice after: %#v", bufioBufSlice)

  // drilling further...

  // bufioRd := bufioReader.FieldByName("rd")
  // log.Printf("bufioRd %#v", bufioRd)
  // log.Printf("bufioRd type %s", bufioRd.Type().String())
  // log.Printf("bufioRd kind %s", bufioRd.Kind().String())
  //
  // bufioRdLimitedPtr := bufioRd.Elem()
  // log.Printf("bufioRd value %#v", bufioRdLimitedPtr)
  // log.Printf("bufioRd value type %s", bufioRdLimitedPtr.Type().String())
  // log.Printf("bufioRd value kind %s", bufioRdLimitedPtr.Kind().String())
  //
  // bufioRdLimited := bufioRdLimitedPtr.Elem()
  // log.Printf("bufioRdLimited value %#v", bufioRdLimited)
  // log.Printf("bufioRdLimited value type %s", bufioRdLimited.Type().String())
  // log.Printf("bufioRdLimited value kind %s", bufioRdLimited.Kind().String())
  //
  // bufioRdLimitedSrcInt := bufioRdLimited.FieldByName("R")
  // log.Printf("bufioRdLimitedSrcInt %#v", bufioRdLimitedSrcInt)
  // log.Printf("bufioRdLimitedSrcInt type %s", bufioRdLimitedSrcInt.Type().String())
  // log.Printf("bufioRdLimitedSrcInt kind %s", bufioRdLimitedSrcInt.Kind().String())
  //
  // bufioRdLimitedSrc := bufioRdLimitedSrcInt.Elem()
  // log.Printf("bufioRdLimitedSrc %#v", bufioRdLimitedSrc)
  // log.Printf("bufioRdLimitedSrc type %s", bufioRdLimitedSrc.Type().String())
  // log.Printf("bufioRdLimitedSrc kind %s", bufioRdLimitedSrc.Kind().String())
  //
  // httpLiveSwitchReader := bufioRdLimitedSrc.Elem()
  // log.Printf("httpLiveSwitchReader %#v", httpLiveSwitchReader)
  // log.Printf("httpLiveSwitchReader type %s", httpLiveSwitchReader.Type().String())
  // log.Printf("httpLiveSwitchReader kind %s", httpLiveSwitchReader.Kind().String())
  //
  // httpLiveSwitchReaderR := httpLiveSwitchReader.FieldByName("r")
  // log.Printf("httpLiveSwitchReaderR %#v", httpLiveSwitchReaderR)
  // log.Printf("httpLiveSwitchReaderR type %s", httpLiveSwitchReaderR.Type().String())
  // log.Printf("httpLiveSwitchReaderR kind %s", httpLiveSwitchReaderR.Kind().String())
  //
  // httpLiveSwitchReaderRValue := httpLiveSwitchReaderR.Elem()
  // log.Printf("httpLiveSwitchReaderRValue %#v", httpLiveSwitchReaderRValue)
  // log.Printf("httpLiveSwitchReaderRValue type %s", httpLiveSwitchReaderRValue.Type().String())
  // log.Printf("httpLiveSwitchReaderRValue kind %s", httpLiveSwitchReaderRValue.Kind().String())
  //
  // tcpConn := httpLiveSwitchReaderRValue.Elem()
  // log.Printf("tcpConn %#v", tcpConn)
  // log.Printf("tcpConn type %s", tcpConn.Type().String())
  // log.Printf("tcpConn kind %s", tcpConn.Kind().String())
  //
  // // typeOfTcpConn := tcpConn.Type()
  // // for i := 0; i < tcpConn.NumField(); i++ {
  // //     f := tcpConn.Field(i)
  // //     log.Printf("%d: %s %s\n", i, typeOfTcpConn.Field(i).Name, f.Type())
  // // }
  //
  // netConn := tcpConn.FieldByName("conn")
  // log.Printf("netConn %#v", netConn)
  // log.Printf("netConn type %s", netConn.Type().String())
  // log.Printf("netConn kind %s", netConn.Kind().String())
  //
  // netConnFdPtr := netConn.FieldByName("fd")
  // log.Printf("netConnFdPtr %#v", netConnFdPtr)
  // log.Printf("netConnFdPtr type %s", netConnFdPtr.Type().String())
  // log.Printf("netConnFdPtr kind %s", netConnFdPtr.Kind().String())
  //
  // netConnFd := netConnFdPtr.Elem()
  // log.Printf("netConnFd %#v", netConnFd)
  // log.Printf("netConnFd type %s", netConnFd.Type().String())
  // log.Printf("netConnFd kind %s", netConnFd.Kind().String())
  //
  // // I think this is the bottom...
  //
  // // type netFD struct {
  // //   // locking/lifetime of sysfd + serialize access to Read and Write methods
  // //   fdmu fdMutex
  // //
  // //   // immutable until Close
  // //   sysfd       int
  // //   family      int
  // //   sotype      int
  // //   isConnected bool
  // //   net         string
  // //   laddr       Addr
  // //   raddr       Addr
  // //
  // //   // wait server
  // //   pd pollDesc
  // // }
  // //
}

func zeroResponseBuffer(response http.ResponseWriter) {
  responseWriter := reflect.ValueOf(response).Elem()
  // log.Printf("responseWriter %#v", responseWriter)
  // log.Printf("responseWriter type %s", responseWriter.Type().String())
  // log.Printf("responseWriter kind %s", responseWriter.Kind().String())

  bufioWriterPtr := responseWriter.FieldByName("w")
  // log.Printf("bufioWriterPtr %#v", bufioWriterPtr)
  // log.Printf("bufioWriterPtr type %s", bufioWriterPtr.Type().String())
  // log.Printf("bufioWriterPtr kind %s", bufioWriterPtr.Kind().String())

  // Force a flush!

  bufioWriterPtrForRealz := (*bufio.Writer)((unsafe.Pointer)(bufioWriterPtr.Pointer()))
  bufioWriterPtrForRealz.Flush()

  // Clear the buffer

  bufioWriter := bufioWriterPtr.Elem()
  // log.Printf("bufioWriter %#v", bufioWriter)
  // log.Printf("bufioWriter type %s", bufioWriter.Type().String())
  // log.Printf("bufioWriter kind %s", bufioWriter.Kind().String())

  bufioBuf := bufioWriter.FieldByName("buf")

  bufioBufSlice := bufioBuf.Bytes()

  zeroBuffer(bufioBufSlice)

  // httpChunkWriterPtr := bufioWriter.FieldByName("wr").Elem()
  // log.Printf("httpChunkWriterPtr %#v", httpChunkWriterPtr)
  // log.Printf("httpChunkWriterPtr type %s", httpChunkWriterPtr.Type().String())
  // log.Printf("httpChunkWriterPtr kind %s", httpChunkWriterPtr.Kind().String())

  // httpChunkWriter := httpChunkWriterPtr.Elem()
  // log.Printf("httpChunkWriter %#v", httpChunkWriter)
  // log.Printf("httpChunkWriter type %s", httpChunkWriter.Type().String())
  // log.Printf("httpChunkWriter kind %s", httpChunkWriter.Kind().String())

  conn := responseWriter.FieldByName("conn").Elem()
  connBufioReadWriter := conn.FieldByName("buf").Elem()
  connBufioWriterPtr := connBufioReadWriter.FieldByName("Writer")

  // Flush conn's buffered io

  connBufioWriterPtrForRealz := (*bufio.Writer)((unsafe.Pointer)(connBufioWriterPtr.Pointer()))
  connBufioWriterPtrForRealz.Flush()

  // Clear conn's buffer

  connBufioWriter := connBufioWriterPtr.Elem()

  connBufioWriterBuf := connBufioWriter.FieldByName("buf")

  connBufioWriterBufSlice := connBufioWriterBuf.Bytes()

  zeroBuffer(connBufioWriterBufSlice)
}

func zeroBuffer(buf []byte) {
  for i := 0; i < len(buf); i++ {
    buf[i] = 0
  }
}