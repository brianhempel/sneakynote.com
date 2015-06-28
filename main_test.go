package main_test

import (
  "bytes"
  "github.com/brianhempel/sneakynote.com"
  "github.com/brianhempel/sneakynote.com/store"
  "io"
  "io/ioutil"
  "os"
  "os/exec"
  "path"
  "strings"
  "net/http"
  "net/http/httptest"
  "regexp"
  "runtime/debug"
  "time"
  "testing"
)

func TestRoot(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()

  response, err := http.Get(testServer.URL)

  if err != nil {
    t.Error(err)
  }

  if response.StatusCode != 200 {
    t.Errorf("Expected status 200, got %d", response.StatusCode)
  }

  defer response.Body.Close()
  body, _ := ioutil.ReadAll(response.Body)

  if !strings.Contains(string(body), "SneakyNote.com") {
    t.Errorf("Expected to find \"SneakyNote.com\" in %s", body)
  }
}

func TestGetSend(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()

  response, err := http.Get(testServer.URL + "/send")

  if err != nil {
    t.Error(err)
  }

  if response.StatusCode != 200 {
    t.Errorf("Expected status 200, got %d", response.StatusCode)
  }

  defer response.Body.Close()
  body, _ := ioutil.ReadAll(response.Body)

  if !strings.Contains(string(body), "Send a SneakyNote") {
    t.Errorf("Expected to find \"Send a SneakyNote\" in %s", body)
  }
}

func TestPostNote(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  reqBodyReader := strings.NewReader("this is my secret")
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 201 {
    t.Errorf("Expected status 201, got %d", response.StatusCode)
  }

  codeRegexp := regexp.MustCompile("\\A[2-9a-hj-km-np-tv-z]{3} [2-9a-hj-km-np-tv-z]{3} [2-9a-hj-km-np-tv-z]{4}\\z")

  if !codeRegexp.MatchString(response.Header.Get("X-Note-Code")) {
    t.Errorf("Expected \"X-Note-Code\" to look like a human code, got %s", response.Header.Get("X-Note-Code"))
  }
}

func TestPostNoteSecretTooLarge(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  secret := strings.Repeat("x", 1024*16 + 1)
  reqBodyReader := strings.NewReader(secret)
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 413 {
    t.Errorf("Expected status 413, got %d", response.StatusCode)
  }

  if response.Header.Get("Content-Type") != "application/json" {
    t.Errorf("Expected \"Content-Type: application-json\", got %s", response.Header.Get("Content-Type"))
  }

  defer response.Body.Close()
  body, _ := ioutil.ReadAll(response.Body)

  if !strings.Contains(string(body), "\"error_type\": \"secret_too_large\"") {
    t.Errorf("Expected to find \"\"error_type\": \"secret_too_large\"\" in %s", body)
  }

  if !strings.Contains(string(body), "\"error_message\": \"Secret too large. Maximum allowed secret size is 16384 bytes.\"") {
    t.Errorf("Expected to find \"\"error_message\": \"Secret too large. Maximum allowed secret size is 16384 bytes.\"\" in %s", body)
  }
}

func TestPostNoteDuplicateUuid(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  reqBodyReader := strings.NewReader("this is my secret")
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  reqBodyReader = strings.NewReader("this is my secret")
  response, err = http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 403 {
    t.Errorf("Expected status 403, got %d", response.StatusCode)
  }

  if response.Header.Get("Content-Type") != "application/json" {
    t.Errorf("Expected \"Content-Type: application-json\", got %s", response.Header.Get("Content-Type"))
  }

  defer response.Body.Close()
  body, _ := ioutil.ReadAll(response.Body)

  if !strings.Contains(string(body), "\"error_type\": \"duplicate_id\"") {
    t.Errorf("Expected to find \"\"error_type\": \"duplicate_id\"\" in %s", body)
  }

  if !strings.Contains(string(body), "\"error_message\": \"A secret with that ID has already been created. If you are not an attacker trying to replace the secret, this indicates a bug in your program and a potentially insecure source of randomness. As a precaution/penalty, the secret has been destroyed (if it has not already expired or been accessed).\"") {
    t.Errorf("Expected to find \"\"error_message\": \"A secret with that ID has already been created. If you are not an attacker trying to replace the secret, this indicates a bug in your program and a potentially insecure source of randomness. As a precaution/penalty, the secret has been destroyed (if it has not already expired or been accessed).\"\" in %s", body)
  }

  defer response.Body.Close()
  response, err = http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 403 {
    t.Errorf("Expected status 403, got %d", response.StatusCode)
  }

  body, _ = ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }
}

func TestPostNoteDuplicateUuidExpiredSecret(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  reqBodyReader := strings.NewReader("this is my secret")
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  // Move the file to the expired folder
  // (Usually an empty file is put there but this is fine.)

  files, err := ioutil.ReadDir(store.DefaultStorePath)
  if err != nil {
    t.Error(err)
    return
  }

  for _, fileInfo := range files {
    if !fileInfo.IsDir() {
      filePath := path.Join(store.DefaultStorePath, fileInfo.Name())
      expiredPath := path.Join(store.DefaultStorePath, "expired", fileInfo.Name())
      os.Rename(filePath, expiredPath)
      if err != nil {
        t.Error(err)
      }
    }
  }

  reqBodyReader = strings.NewReader("this is my secret")
  response, err = http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 403 {
    t.Errorf("Expected status 403, got %d", response.StatusCode)
  }

  if response.Header.Get("Content-Type") != "application/json" {
    t.Errorf("Expected \"Content-Type: application-json\", got %s", response.Header.Get("Content-Type"))
  }

  defer response.Body.Close()
  body, _ := ioutil.ReadAll(response.Body)

  if !strings.Contains(string(body), "\"error_type\": \"duplicate_id\"") {
    t.Errorf("Expected to find \"\"error_type\": \"duplicate_id\"\" in %s", body)
  }

  if !strings.Contains(string(body), "\"error_message\": \"A secret with that ID has already been created. If you are not an attacker trying to replace the secret, this indicates a bug in your program and a potentially insecure source of randomness. As a precaution/penalty, the secret has been destroyed (if it has not already expired or been accessed).\"") {
    t.Errorf("Expected to find \"\"error_message\": \"A secret with that ID has already been created. If you are not an attacker trying to replace the secret, this indicates a bug in your program and a potentially insecure source of randomness. As a precaution/penalty, the secret has been destroyed (if it has not already expired or been accessed).\"\" in %s", body)
  }

  defer response.Body.Close()
  response, err = http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 410 {
    t.Errorf("Expected status 410, got %d", response.StatusCode)
  }

  body, _ = ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }
}

func TestPostNoteDuplicateUuidAccessedSecret(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  reqBodyReader := strings.NewReader("this is my secret")
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  // Move the file to the accessed folder
  // (Usually an empty file is put there but this is fine.)

  files, err := ioutil.ReadDir(store.DefaultStorePath)
  if err != nil {
    t.Error(err)
    return
  }

  for _, fileInfo := range files {
    if !fileInfo.IsDir() {
      filePath := path.Join(store.DefaultStorePath, fileInfo.Name())
      accessedPath := path.Join(store.DefaultStorePath, "accessed", fileInfo.Name())
      os.Rename(filePath, accessedPath)
      if err != nil {
        t.Error(err)
      }
    }
  }

  reqBodyReader = strings.NewReader("this is my secret")
  response, err = http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 403 {
    t.Errorf("Expected status 403, got %d", response.StatusCode)
  }

  if response.Header.Get("Content-Type") != "application/json" {
    t.Errorf("Expected \"Content-Type: application-json\", got %s", response.Header.Get("Content-Type"))
  }

  defer response.Body.Close()
  body, _ := ioutil.ReadAll(response.Body)

  if !strings.Contains(string(body), "\"error_type\": \"duplicate_id\"") {
    t.Errorf("Expected to find \"\"error_type\": \"duplicate_id\"\" in %s", body)
  }

  if !strings.Contains(string(body), "\"error_message\": \"A secret with that ID has already been created. If you are not an attacker trying to replace the secret, this indicates a bug in your program and a potentially insecure source of randomness. As a precaution/penalty, the secret has been destroyed (if it has not already expired or been accessed).\"") {
    t.Errorf("Expected to find \"\"error_message\": \"A secret with that ID has already been created. If you are not an attacker trying to replace the secret, this indicates a bug in your program and a potentially insecure source of randomness. As a precaution/penalty, the secret has been destroyed (if it has not already expired or been accessed).\"\" in %s", body)
  }

  defer response.Body.Close()
  response, err = http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 403 {
    t.Errorf("Expected status 403, got %d", response.StatusCode)
  }

  body, _ = ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }
}

func TestPostNoteSecretStorageFull(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  var secret string
  var reqBodyReader io.Reader
  var response *http.Response
  var err error

  for i := 0; i < (1024/16); i++ {
    secret = strings.Repeat("x", 1024*16)
    reqBodyReader = strings.NewReader(secret)
    response, err = http.Post(testServer.URL + "/notes/" + store.GenerateUuid(), "application/octet-stream", reqBodyReader)
  }

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 507 {
    t.Errorf("Expected status 507, got %d", response.StatusCode)
  }

  if response.Header.Get("Content-Type") != "application/json" {
    t.Errorf("Expected \"Content-Type: application-json\", got %s", response.Header.Get("Content-Type"))
  }

  defer response.Body.Close()
  body, _ := ioutil.ReadAll(response.Body)

  if !strings.Contains(string(body), "\"error_type\": \"storage_full\"") {
    t.Errorf("Expected to find \"\"error_type\": \"storage_full\"\" in %s", body)
  }

  if !strings.Contains(string(body), "\"error_message\": \"Sorry, server secret storage is full right now. Try again later.\"") {
    t.Errorf("Expected to find \"\"error_message\": \"Sorry, server secret storage is full right now. Try again later.\"\" in %s", body)
  }
}

func TestPostNoteSecretClearedFromMemory(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  err := exec.Command("sh", "-c", "head -c 16 /dev/urandom > /tmp/random_secret").Run()
  defer os.Remove("/tmp/random_secret")
  if err != nil {
    t.Error("Error generating random secret:", err)
    return
  }

  err = exec.Command("curl", "-d", "@/tmp/random_secret", testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27").Run()
  if err != nil {
    t.Error("Error POSTing random secret:", err)
    return
  }

  // Do a heap dump to see if the secret was cleared from our memory

  f, err := os.Create("/tmp/heapdump")
  if err != nil {
    t.Errorf("Error creating /tmp/heapdump: %s", err)
  }
  defer os.Remove("/tmp/heapdump")

  debug.WriteHeapDump(f.Fd())
  f.Close()

  dump, err := ioutil.ReadFile("/tmp/heapdump")
  if err != nil {
    t.Errorf("Error reading /tmp/heapdump: %s", err)
  }

  randomSecret, err := ioutil.ReadFile("/tmp/random_secret")
  if err != nil {
    t.Errorf("Error reading random secret: %s", err)
  }

  if bytes.Contains(dump, randomSecret) {
    t.Error("Saved data not cleared from memory", randomSecret)
  }
}

func TestGetNote(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  secret := "this is my secret"
  reqBodyReader := strings.NewReader(secret)
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  } else if response.StatusCode != 201 {
    t.Errorf("Expected status 201, got %d", response.StatusCode)
  }

  code := response.Header.Get("X-Note-Code")

  defer response.Body.Close()
  response, err = http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 200 {
    t.Errorf("Expected status 200, got %d", response.StatusCode)
  }

  body, _ := ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte(secret)) {
    t.Errorf("Expected returned data to be %s, got %s", secret, string(body))
  }

  if code != response.Header.Get("X-Note-Code") {
    t.Errorf("Expected X-Note-Code to contain original code %s, got %s", code, response.Header.Get("X-Note-Code"))
  }

  // Second time should be 403 Forbidden (secret has been accessed or is being
  // accessed)

  defer response.Body.Close()
  response, err = http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 403 {
    t.Errorf("Expected status 403, got %d", response.StatusCode)
  }

  body, _ = ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }
}

func TestGetNoteExpired(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  reqBodyReader := strings.NewReader("this is my secret")
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  } else if response.StatusCode != 201 {
    t.Errorf("Expected status 201, got %d", response.StatusCode)
  }

  // Hack the file to be old...

  files, err := ioutil.ReadDir(store.DefaultStorePath)
  if err != nil {
    t.Error(err)
    return
  }

  for _, fileInfo := range files {
    if !fileInfo.IsDir() {
      oldTime := time.Now().Add(-10*time.Minute)
      filePath := path.Join(store.DefaultStorePath, fileInfo.Name())
      os.Chtimes(filePath, time.Now(), oldTime)
      if err != nil {
        t.Error(err)
      }
    }
  }

  // Make sure it comes back as expired

  defer response.Body.Close()
  response, err = http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 410 {
    t.Errorf("Expected status 410, got %d", response.StatusCode)
  }

  body, _ := ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }
}

func TestGetNoteNotFound(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  response, err := http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")
  defer response.Body.Close()

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 404 {
    t.Errorf("Expected status 404, got %d", response.StatusCode)
  }

  body, _ := ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }
}

func TestGetNoteSecretClearedFromMemory(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  err := exec.Command("sh", "-c", "head -c 16 /dev/urandom > /tmp/random_secret").Run()
  defer os.Remove("/tmp/random_secret")
  if err != nil {
    t.Error("Error generating random secret:", err)
    return
  }

  out, err := exec.Command("curl", "-d", "@/tmp/random_secret", testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27").Output()
  if err != nil {
    t.Error("Error POSTing random secret:", err, out)
    return
  }

  // We already ensure the secret disappears on create, now get it
  // to ensure it desappears on access too

  err = exec.Command("curl", testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27").Run()
  if err != nil {
    t.Error("Error GETing secret:", err)
    return
  }

  // Do a heap dump to see if the secret was cleared from our memory

  f, err := os.Create("/tmp/heapdump")
  if err != nil {
    t.Errorf("Error creating /tmp/heapdump: %s", err)
  }
  defer os.Remove("/tmp/heapdump")

  debug.WriteHeapDump(f.Fd())
  f.Close()

  dump, err := ioutil.ReadFile("/tmp/heapdump")
  if err != nil {
    t.Errorf("Error reading /tmp/heapdump: %s", err)
  }

  randomSecret, err := ioutil.ReadFile("/tmp/random_secret")
  if err != nil {
    t.Errorf("Error reading random secret: %s", err)
  }

  if bytes.Contains(dump, randomSecret) {
    t.Error("Saved data not cleared from memory", randomSecret)
  }
}

func TestGetNoteStatus(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  secret := "this is my secret"
  reqBodyReader := strings.NewReader(secret)
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  } else if response.StatusCode != 201 {
    t.Errorf("Expected status 201, got %d", response.StatusCode)
  }

  code := response.Header.Get("X-Note-Code")

  url := testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27/status"
  request, err := http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", code)

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 200 {
    t.Errorf("Expected status 200, got %d", response.StatusCode)
  }

  body, _ := ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }

  // 404 if code doesn't match

  request, err = http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", "bad code")

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 404 {
    t.Errorf("Expected status 404, got %d", response.StatusCode)
  }
}

func TestGetNoteStatusAccessed(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  secret := "this is my secret"
  reqBodyReader := strings.NewReader(secret)
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  } else if response.StatusCode != 201 {
    t.Errorf("Expected status 201, got %d", response.StatusCode)
  }

  // Access the secret.

  defer response.Body.Close()
  response, err = http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")

  if err != nil {
    t.Error(err)
    return
  }

  // Ask for status

  code := response.Header.Get("X-Note-Code")

  url := testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27/status"
  request, err := http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", code)

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 403 {
    t.Errorf("Expected status 403, got %d", response.StatusCode)
  }

  body, _ := ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }

  // 404 if code doesn't match

  request, err = http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", "bad code")

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 404 {
    t.Errorf("Expected status 404, got %d", response.StatusCode)
  }
}

func TestGetNoteStatusExpiredSecretNotSwept(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  secret := "this is my secret"
  reqBodyReader := strings.NewReader(secret)
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  } else if response.StatusCode != 201 {
    t.Errorf("Expected status 201, got %d", response.StatusCode)
  }

  code := response.Header.Get("X-Note-Code")

  // Hack the file to be old...

  files, err := ioutil.ReadDir(store.DefaultStorePath)
  if err != nil {
    t.Error(err)
    return
  }

  for _, fileInfo := range files {
    if !fileInfo.IsDir() {
      oldTime := time.Now().Add(-10*time.Minute)
      filePath := path.Join(store.DefaultStorePath, fileInfo.Name())
      os.Chtimes(filePath, time.Now(), oldTime)
      if err != nil {
        t.Error(err)
      }
    }
  }

  // Ask for status

  url := testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27/status"
  request, err := http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", code)

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 410 {
    t.Errorf("Expected status 410, got %d", response.StatusCode)
  }

  body, _ := ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }

  // 404 if code doesn't match

  request, err = http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", "bad code")

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 404 {
    t.Errorf("Expected status 404, got %d", response.StatusCode)
  }
}

func TestGetNoteStatusExpiredSecretSwept(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  secret := "this is my secret"
  reqBodyReader := strings.NewReader(secret)
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  } else if response.StatusCode != 201 {
    t.Errorf("Expected status 201, got %d", response.StatusCode)
  }

  code := response.Header.Get("X-Note-Code")

  // Make an expired record

  files, err := ioutil.ReadDir(store.DefaultStorePath)
  if err != nil {
    t.Error(err)
    return
  }

  // Should only be one file
  for _, fileInfo := range files {
    if !fileInfo.IsDir() {
      expiredFilePath := path.Join(store.DefaultStorePath, "expired", fileInfo.Name())
      ioutil.WriteFile(expiredFilePath, []byte(code), 0400)
    }
  }

  // Ask for status

  url := testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27/status"
  request, err := http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", code)

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 410 {
    t.Errorf("Expected status 410, got %d", response.StatusCode)
  }

  body, _ := ioutil.ReadAll(response.Body)

  if !bytes.Equal(body, []byte("")) {
    t.Errorf("Expected returned data to be blank, got %s", string(body))
  }

  // 404 if code doesn't match

  request, err = http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", "bad code")

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 404 {
    t.Errorf("Expected status 404, got %d", response.StatusCode)
  }
}

func TestGetNoteStatusNotFound(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  code := "234 567 abcd"

  url := testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27/status"
  request, err := http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", code)

  response, err := http.DefaultClient.Do(request)
  defer response.Body.Close()

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 404 {
    t.Errorf("Expected status 404, got %d", response.StatusCode)
  }
}

func TestGetNoteStatusBadUuid(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  code := "234 567 abcd"

  url := testServer.URL + "/notes/asdf/status"
  request, err := http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", code)

  response, err := http.DefaultClient.Do(request)
  defer response.Body.Close()

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 404 {
    t.Errorf("Expected status 404, got %d", response.StatusCode)
  }
}

func TestGetNoteStatusLongPoll(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()
  main.SetupStore()
  defer main.TeardownStore()

  secret := "this is my secret"
  reqBodyReader := strings.NewReader(secret)
  response, err := http.Post(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "application/octet-stream", reqBodyReader)

  if err != nil {
    t.Error(err)
    return
  } else if response.StatusCode != 201 {
    t.Errorf("Expected status 201, got %d", response.StatusCode)
  }

  code := response.Header.Get("X-Note-Code")

  // Access the secret after half a second.
  go func() {
    time.Sleep(time.Millisecond * 500)

    response, _ := http.Get(testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27")
    response.Body.Close()
  }()

  // Ask for status

  url := testServer.URL + "/notes/fc2a4122-e81e-4b10-a31b-d79fbdb33a27/status"
  request, err := http.NewRequest("GET", url, nil)
  if err != nil {
    t.Error(err)
    return
  }
  request.Header.Set("X-Note-Code", code)
  request.Header.Set("X-Long-Poll", "true")

  defer response.Body.Close()
  response, err = http.DefaultClient.Do(request)

  if err != nil {
    t.Error(err)
    return
  }

  if response.StatusCode != 403 {
    t.Errorf("Expected status 403, got %d", response.StatusCode)
  }
}

func TestGetFreeSpace(t *testing.T) {
  testServer := httptest.NewServer(main.Handlers())
  defer testServer.Close()

  response, err := http.Get(testServer.URL + "/free_space")

  if err != nil {
    t.Error(err)
  }

  if response.StatusCode != 200 {
    t.Errorf("Expected status 200, got %d", response.StatusCode)
  }

  defer response.Body.Close()
  body, _ := ioutil.ReadAll(response.Body)

  if !strings.Contains(string(body), "MB") {
    t.Errorf("Expected to find \"MB\" in %s", body)
  }
}
