package store_test

import (
  "github.com/brianhempel/sneakynote.com/store"
  "bytes"
  "crypto/sha256"
  "encoding/hex"
  "io/ioutil"
  "os"
  "os/exec"
  "path"
  "regexp"
  "runtime/debug"
  "strings"
  "testing"
  "time"
)

func TestSetup(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  hdiutilInfo, err := exec.Command("hdiutil", "info").Output()
  if err != nil {
    t.Errorf("udiutil info:", err)
  }
  hdiutilInfoStr := string(hdiutilInfo)

  if !strings.Contains(hdiutilInfoStr, s.Root) {
    t.Errorf("Expected %s to be linked. hdiutil output: %s", s.Root, hdiutilInfoStr)
  }

  if _, err = os.Stat(s.Root); os.IsNotExist(err) {
    t.Errorf("Expected folder %s to exist", s.Root)
  }

  testFilePath := path.Join(s.Root, "test_file")
  err = ioutil.WriteFile(testFilePath, []byte("some stuff"), 0600)
  if err != nil {
    t.Errorf("Expected %s folder to be writable: %s", s.Root, err)
  } else {
    os.Remove(testFilePath)
  }

  testFilePath = path.Join(s.BeingAccessedPath, "test_file")
  err = ioutil.WriteFile(testFilePath, []byte("some stuff"), 0600)
  if err != nil {
    t.Errorf("Expected %s folder to be writable: %s", s.BeingAccessedPath, err)
  } else {
    os.Remove(testFilePath)
  }

  testFilePath = path.Join(s.AccessedPath, "test_file")
  err = ioutil.WriteFile(testFilePath, []byte("some stuff"), 0600)
  if err != nil {
    t.Errorf("Expected %s folder to be writable: %s", s.AccessedPath, err)
  } else {
    os.Remove(testFilePath)
  }

  testFilePath = path.Join(s.ExpiringPath, "test_file")
  err = ioutil.WriteFile(testFilePath, []byte("some stuff"), 0600)
  if err != nil {
    t.Errorf("Expected %s folder to be writable: %s", s.ExpiredPath, err)
  } else {
    os.Remove(testFilePath)
  }

  testFilePath = path.Join(s.ExpiredPath, "test_file")
  err = ioutil.WriteFile(testFilePath, []byte("some stuff"), 0600)
  if err != nil {
    t.Errorf("Expected %s folder to be writable: %s", s.ExpiredPath, err)
  } else {
    os.Remove(testFilePath)
  }
}

func TestTeardown(t *testing.T) {
  s := store.Setup()
  s.Teardown()

  if _, err := os.Stat(s.Root); !os.IsNotExist(err) {
    t.Errorf("Expected folder %s not to exist", s.Root)
  }

  hdiutilInfo, err := exec.Command("hdiutil", "info").Output()
  if err != nil {
    t.Errorf("udiutil info:", err)
  }
  hdiutilInfoStr := string(hdiutilInfo)

  if matched, _ := regexp.MatchString(s.Root + "\\b", hdiutilInfoStr); matched {
    t.Errorf("Expected %s to be ejected. hdiutil output: %s", s.Root, hdiutilInfoStr)
  }
}

func TestSave(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  testData := []byte("saved test data 123")

  reader := bytes.NewReader(testData)
  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  code, err := s.Save(reader, id)
  if err != nil {
    t.Errorf("Error on store.Save: %s", err)
  }

  codeRegexp := regexp.MustCompile("\\A[2-9a-hj-km-np-tv-z]{3} [2-9a-hj-km-np-tv-z]{3} [2-9a-hj-km-np-tv-z]{4}\\z")

  if !codeRegexp.MatchString(code) {
    t.Errorf("Expected code to look like a human code, got %s", code)
  }

  // Test that the secret was wiped from memory

  // Obfuscate the copy in the tests...
  for i := 0; i < len(testData); i++ {
    testData[i] = testData[i] ^ 255
  }

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

  // Deobfuscate the copy in the tests...
  for i := 0; i < len(testData); i++ {
    testData[i] = testData[i] ^ 255
  }

  if bytes.Contains(dump, testData) {
    t.Errorf("Saved data %s not cleared from memory", string(testData))
  }

  // Test that everything stored correctly
  idBytes, _ := hex.DecodeString(strings.Replace(id, "-", "", -1))
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  filePath := path.Join(s.Root, fileName)

  savedBytes, err := ioutil.ReadFile(filePath)
  if err != nil {
    t.Errorf("Error reading save data: %s", err)
  }
  if !bytes.Equal(append([]byte(code + "\n"), testData...), savedBytes) {
    t.Errorf("Expected the saved data %s to equal %s", string(savedBytes), string(testData))
  }
}

func TestSaveTooBig(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  testDataRightSize := make([]byte, store.DefaultMaxSecretSize)

  reader := bytes.NewReader(testDataRightSize)
  _, err := s.Save(reader, store.GenerateUuid())
  if err != nil {
    t.Errorf("Error on store.Save: %s", err)
  }

  testDataWrongSize := make([]byte, store.DefaultMaxSecretSize + 1)

  reader = bytes.NewReader(testDataWrongSize)
  _, err = s.Save(reader, store.GenerateUuid())
  expectedError := "Secret too large"
  if err == nil {
    t.Errorf("Error expected on store.Save with too much data. No error returned")
  } else if !strings.Contains(err.Error(), expectedError) {
    t.Errorf("Error on store.Save, expected error to contain %#s but got %#s", expectedError, err.Error())
  }
}

func TestSaveDuplicateIdNotAccessed(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  id := store.GenerateUuid()
  fileName := s.UuidToFileName(id)
  secretPath := path.Join(s.Root, fileName)

  err := ioutil.WriteFile(secretPath, []byte("234 567 abcd\n"), 0600)
  if err != nil {
    t.Error("Error making file", err)
  }

  reader := bytes.NewReader(make([]byte, 0))
  _, err = s.Save(reader, id)
  expectedError := "ID has been used before"
  if err == nil {
    t.Errorf("Error expected on store.Save with duplicate id. No error returned")
  } else if !strings.Contains(err.Error(), expectedError) {
    t.Errorf("Error on store.Save, expected error to contain %#s but got %#s", expectedError, err.Error())
  }

  if _, err := os.Stat(secretPath); !os.IsNotExist(err) {
    t.Errorf("Expected secret file %s to not exist, but was found.", secretPath)
  }

  accessedPath := path.Join(s.AccessedPath, fileName)

  if _, err := os.Stat(accessedPath); err != nil {
    t.Errorf("Expected accessed record %s to exist, but was not found. %#s", accessedPath, err.Error())
  }

  savedBytes, err := ioutil.ReadFile(accessedPath)
  if err != nil {
    t.Error("Error reading accessed file", err)
  }
  if !bytes.Equal(savedBytes, []byte("234 567 abcd")) {
    t.Error("Expected accessed file to contain the secret code \"secret code\", but contained", savedBytes)
  }
}

func TestSaveDuplicateIdAccessed(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  id := store.GenerateUuid()
  fileName := s.UuidToFileName(id)
  secretPath := path.Join(s.Root, fileName)
  accessedPath := path.Join(s.AccessedPath, fileName)

  err := ioutil.WriteFile(accessedPath, nil, 0400)
  if err != nil {
    t.Error("Error making file", err)
  }

  reader := bytes.NewReader(make([]byte, 0))
  _, err = s.Save(reader, id)
  expectedError := "ID has been used before"
  if err == nil {
    t.Errorf("Error expected on store.Save with duplicate id. No error returned")
  } else if !strings.Contains(err.Error(), expectedError) {
    t.Errorf("Error on store.Save, expected error to contain %#s but got %#s", expectedError, err.Error())
  }

  if _, err := os.Stat(secretPath); !os.IsNotExist(err) {
    t.Errorf("Expected secret file %s to not exist, but was found.", secretPath)
  }
}

func TestSaveDuplicateIdExpired(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  id := store.GenerateUuid()
  fileName := s.UuidToFileName(id)
  secretPath := path.Join(s.Root, fileName)
  expiredPath := path.Join(s.ExpiredPath, fileName)

  err := ioutil.WriteFile(expiredPath, nil, 0400)
  if err != nil {
    t.Error("Error making file", err)
  }

  reader := bytes.NewReader(make([]byte, 0))
  _, err = s.Save(reader, id)
  expectedError := "ID has been used before"
  if err == nil {
    t.Errorf("Error expected on store.Save with duplicate id. No error returned")
  } else if !strings.Contains(err.Error(), expectedError) {
    t.Errorf("Error on store.Save, expected error to contain %#s but got %#s", expectedError, err.Error())
  }

  if _, err := os.Stat(secretPath); !os.IsNotExist(err) {
    t.Errorf("Expected secret file %s to not exist, but was found.", secretPath)
  }
}

func TestSaveOutOfMemory(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  available := s.AvailableMemory()
  s.MaxSecretSize = available + 1024

  testData := make([]byte, s.MaxSecretSize)

  reader := bytes.NewReader(testData)
  _, err := s.Save(reader, store.GenerateUuid())
  expectedError := "Secret storage full"
  if err == nil {
    t.Errorf("Error expected on store.Save with no memory available. No error returned")
  } else if !strings.Contains(err.Error(), expectedError) {
    t.Errorf("Error on store.Save, expected error to contain %#s but got %#s", expectedError, err.Error())
  }
}

func TestRetrieve(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  testCode := "234 567 abcd"
  testData := []byte("saved test data 123")
  fileData := []byte(testCode + "\n" + string(testData))
  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  idBytes, err := hex.DecodeString(strings.Replace(id, "-", "", -1))
  if err != nil {
    t.Error("Error converting uuid to bytes:", err, " ", id)
    return
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  filePath := path.Join(s.Root, fileName)

  err = ioutil.WriteFile(filePath, fileData, 0600)
  if err != nil {
    t.Error("Error creating secret file: ", err)
    return
  }

  returnedData := make([]byte, s.MaxSecretSize)

  nRead, code, err := s.Retrieve(id, returnedData)
  if err != nil {
    t.Errorf("Error on store.Retrieve: %s", err)
  }
  if nRead != len(testData) {
    t.Errorf("Expected nRead to be %d but got %d", len(testData), nRead)
  }
  if testCode != code {
    t.Errorf("Expected returned code to be %s but got %s", testCode, code)
  }
  if !bytes.Equal(testData, returnedData[:nRead]) {
    t.Errorf("Expected returned data to be %s but got %s", string(testData), string(returnedData[:nRead]))
  }

  // Original file should not exist

  if _, err := os.Stat(filePath); !os.IsNotExist(err) {
    t.Errorf("Expected secret file %s to not exist, but was found.", filePath)
  }

  // The being_accessed folder should be empty

  files, err := ioutil.ReadDir(s.BeingAccessedPath)
  if err != nil {
    t.Error("Error reading the being_accessed folder:", err)
  }
  if len(files) != 0 {
    t.Error("Expected the being_accessed folder to be empty. It wasn't.")
    for _, fileInfo := range files {
      t.Error(fileInfo.Name())
    }
  }

  // The secret should be logged as accessed and that file should contain only
  // the code

  accessedFilePath := path.Join(s.AccessedPath, fileName)

  savedBytes, err := ioutil.ReadFile(accessedFilePath)
  if err != nil {
    t.Error("Error reading accessed file", err)
  }
  if !bytes.Equal(savedBytes, []byte(testCode)) {
    t.Error("Expected accessed file to contain the secret code \"" + testCode + "\", but contained", savedBytes)
  }

  // // Test that the secret was wiped from memory
  //
  // // Obfuscate the copy in the tests...
  // for i := 0; i < len(testData); i++ {
  //   testData[i] = testData[i] ^ 255
  // }
  //
  // f, err := os.Create("/tmp/heapdump")
  // if err != nil {
  //   t.Errorf("Error creating /tmp/heapdump: %s", err)
  // }
  // debug.WriteHeapDump(f.Fd())
  // f.Close()
  //
  // dump, err := ioutil.ReadFile("/tmp/heapdump")
  // if err != nil {
  //   t.Errorf("Error reading /tmp/heapdump: %s", err)
  // }
  //
  // // Deobfuscate the copy in the tests...
  // for i := 0; i < len(testData); i++ {
  //   testData[i] = testData[i] ^ 255
  // }
  //
  // if bytes.Contains(dump, testData) {
  //   t.Errorf("Saved data %s not cleared from memory", string(testData))
  // }
}

func TestRetrieveAlreadyAccessed(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  idBytes, err := hex.DecodeString(strings.Replace(id, "-", "", -1))
  if err != nil {
    t.Error("Error converting uuid to bytes:", err, " ", id)
    return
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  accesedFilePath := path.Join(s.AccessedPath, fileName)

  err = ioutil.WriteFile(accesedFilePath, []byte("code"), 0400)
  if err != nil {
    t.Error("Error creating accessed file: ", err)
    return
  }

  returnedData := make([]byte, s.MaxSecretSize)

  nRead, _, err := s.Retrieve(id, returnedData)
  if err == nil {
    t.Error("Expected an error on store.Retrieve, got nothing!")
  } else if err != store.SecretAlreadyAccessed {
    t.Error("Expected a SecretAlreadyAccessed error, got", err)
  }

  if nRead != -1 {
    t.Errorf("Expected nRead to be -1 but got %d", nRead)
  }
}

// secret a little old but not yet cleared by sweeper
func TestRetrieveSecretOld(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  idBytes, err := hex.DecodeString(strings.Replace(id, "-", "", -1))
  if err != nil {
    t.Error("Error converting uuid to bytes:", err, " ", id)
    return
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  filePath := path.Join(s.Root, fileName)

  err = ioutil.WriteFile(filePath, []byte("code\nmy super secret"), 0600)
  if err != nil {
    t.Error("Error creating secret file: ", err)
    return
  }
  oldTime := time.Now().Add(-10*time.Minute)
  err = os.Chtimes(filePath, time.Now(), oldTime)

  returnedData := make([]byte, s.MaxSecretSize)

  nRead, code, err := s.Retrieve(id, returnedData)
  if err == nil {
    t.Error("Expected an error on store.Retrieve, got nothing!")
  } else if err != store.SecretExpired {
    t.Error("Expected a SecretExpired error, got", err)
  }

  if nRead != -1 {
    t.Errorf("Expected nRead to be -1 but got %d", nRead)
  }

  if code != "" {
    t.Errorf("Expected code to be \"\" but got %d", code)
  }
}

// Secret cleared by sweeper, record left in expired folder
func TestRetrieveExpired(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  idBytes, err := hex.DecodeString(strings.Replace(id, "-", "", -1))
  if err != nil {
    t.Error("Error converting uuid to bytes:", err, " ", id)
    return
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  expiredFilePath := path.Join(s.ExpiredPath, fileName)

  err = ioutil.WriteFile(expiredFilePath, []byte("code"), 0400)
  if err != nil {
    t.Error("Error creating expired file: ", err)
    return
  }

  returnedData := make([]byte, s.MaxSecretSize)

  nRead, _, err := s.Retrieve(id, returnedData)
  if err == nil {
    t.Error("Expected an error on store.Retrieve, got nothing!")
  } else if err != store.SecretExpired {
    t.Error("Expected a SecretExpired error, got", err)
  }

  if nRead != -1 {
    t.Errorf("Expected nRead to be -1 but got %d", nRead)
  }
}

func TestRetrieveNotFound(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"

  returnedData := make([]byte, s.MaxSecretSize)

  nRead, _, err := s.Retrieve(id, returnedData)
  if err == nil {
    t.Error("Expected an error on store.Retrieve, got nothing!")
  } else if err != store.SecretNotFound {
    t.Error("Expected a SecretNotFound error, got", err)
  }

  if nRead != -1 {
    t.Errorf("Expected nRead to be -1 but got %d", nRead)
  }
}

func TestStatusUnopened(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  testCode := "234 567 abcd"
  testData := []byte("saved test data 123")
  fileData := []byte(testCode + "\n" + string(testData))
  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  idBytes, err := hex.DecodeString(strings.Replace(id, "-", "", -1))
  if err != nil {
    t.Error("Error converting uuid to bytes:", err, " ", id)
    return
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  filePath := path.Join(s.Root, fileName)

  err = ioutil.WriteFile(filePath, fileData, 0600)
  if err != nil {
    t.Error("Error creating secret file: ", err)
    return
  }

  err = s.Status(id, testCode)
  if err != nil {
    t.Error("Expected no error for secret status, got", err)
  }

  // Idempotent

  err = s.Status(id, testCode)
  if err != nil {
    t.Error("Expected no error for secret status, got", err)
  }

  // Codes must match
  err = s.Status(id, "bad code")
  if err != store.SecretNotFound {
    t.Error("Expected a SecretNotFound error for a bad code, got", err)
  }
}

func TestStatusAlreadyAccessed(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  testCode := "234 567 abcd"
  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  idBytes, err := hex.DecodeString(strings.Replace(id, "-", "", -1))
  if err != nil {
    t.Error("Error converting uuid to bytes:", err, " ", id)
    return
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  accesedFilePath := path.Join(s.AccessedPath, fileName)

  err = ioutil.WriteFile(accesedFilePath, []byte(testCode), 0600)
  if err != nil {
    t.Error("Error creating accessed file: ", err)
    return
  }

  err = s.Status(id, testCode)
  if err != store.SecretAlreadyAccessed {
    t.Error("Expected a SecretAlreadyAccessed error, got", err)
  }

  // Idempotent

  err = s.Status(id, testCode)
  if err != store.SecretAlreadyAccessed {
    t.Error("Expected a SecretAlreadyAccessed error, got", err)
  }

  // Requires codes to match
  err = s.Status(id, "bad code")
  if err != store.SecretNotFound {
    t.Error("Expected a SecretNotFound error for a bad code, got", err)
  }
}

// Secret a little old but not yet cleared by sweeper
func TestStatusSecretOld(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  testCode := "234 567 abcd"
  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  idBytes, err := hex.DecodeString(strings.Replace(id, "-", "", -1))
  if err != nil {
    t.Error("Error converting uuid to bytes:", err, " ", id)
    return
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  filePath := path.Join(s.Root, fileName)

  err = ioutil.WriteFile(filePath, []byte(testCode + "\nmy super secret"), 0600)
  if err != nil {
    t.Error("Error creating secret file: ", err)
    return
  }
  oldTime := time.Now().Add(-10*time.Minute)
  err = os.Chtimes(filePath, time.Now(), oldTime)

  err = s.Status(id, testCode)
  if err != store.SecretExpired {
    t.Error("Expected a SecretExpired error, got", err)
  }

  // Idempotent

  err = s.Status(id, testCode)
  if err != store.SecretExpired {
    t.Error("Expected a SecretExpired error, got", err)
  }

  // Requires codes to match

  err = s.Status(id, "bad code")
  if err != store.SecretNotFound {
    t.Error("Expected a SecretNotFound error for a bad code, got", err)
  }
}

// Secret cleared by sweeper, record left in expired folder
func TestStatusExpired(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  testCode := "234 567 abcd"
  id := "fc2a4122-e81e-4b10-a31b-d79fbdb33a27"
  idBytes, err := hex.DecodeString(strings.Replace(id, "-", "", -1))
  if err != nil {
    t.Error("Error converting uuid to bytes:", err, " ", id)
    return
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])
  expiredFilePath := path.Join(s.ExpiredPath, fileName)

  err = ioutil.WriteFile(expiredFilePath, []byte(testCode), 0400)
  if err != nil {
    t.Error("Error creating expired file: ", err)
    return
  }


  err = s.Status(id, testCode)
  if err != store.SecretExpired {
    t.Error("Expected a SecretExpired error, got", err)
  }

  // Idempotent

  err = s.Status(id, testCode)
  if err != store.SecretExpired {
    t.Error("Expected a SecretExpired error, got", err)
  }

  // Requires codes to match

  err = s.Status(id, "bad code")
  if err != store.SecretNotFound {
    t.Error("Expected a SecretNotFound error for a bad code, got", err)
  }
}

func TestStatusNotFound(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  err := s.Status("fc2a4122-e81e-4b10-a31b-d79fbdb33a27", "234 567 abcd")
  if err != store.SecretNotFound {
    t.Error("Expected a SecretNotFound error, got", err)
  }
}