package store_test

import (
  "github.com/brianhempel/sneakynote.com/store"
  "bytes"
  // "crypto/sha256"
  // "encoding/hex"
  "io/ioutil"
  "os"
  // "os/exec"
  "path"
  // "regexp"
  // "runtime/debug"
  // "strings"
  "testing"
  "time"
)

func makeFile(dir string, fileName string, contents string, age time.Duration, permissions int) {
  filePath := path.Join(dir, fileName)
  ioutil.WriteFile(filePath, []byte(contents), 0600)
  os.Chtimes(filePath, time.Now(), time.Now().Add(-age*time.Minute))
}

func TestSweep(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  makeFile(s.Root, "secret_not_old", "234 567 abcd\n", 9, 0600)
  makeFile(s.Root, "secret_old", "234 567 abcd\n", 10, 0600)

  makeFile(s.BeingAccessedPath, "secret_being_accessed_not_old", "234 567 abcd\n", 10, 0600)
  makeFile(s.BeingAccessedPath, "secret_being_accessed_old", "234 567 abcd\n", 11, 0600)

  makeFile(s.AccessedPath, "accessed_record_not_old", "234 567 abcd", 23*60, 0400)
  makeFile(s.AccessedPath, "accessed_record_old", "234 567 abcd", 24*60, 0400)

  makeFile(s.ExpiringPath, "secret_expiring", "234 567 abcd\n", 0, 0600)

  makeFile(s.ExpiredPath, "expired_record_not_old", "234 567 abcd", 23*60, 0400)
  makeFile(s.ExpiredPath, "expired_record_old", "234 567 abcd", 24*60, 0400)

  // Sweep!

  err := s.Sweep()
  if err != nil {
    t.Error("Sweep errored:", err)
  }

  // Test results

  if _, err = os.Stat(path.Join(s.Root, "secret_not_old")); os.IsNotExist(err) {
    t.Error("Expected secret_not_old to exist but does not")
  }

  if _, err = os.Stat(path.Join(s.Root, "secret_old")); !os.IsNotExist(err) {
    t.Error("Expected secret_old not to exist", err)
  }

  savedBytes, err := ioutil.ReadFile(path.Join(s.ExpiredPath, "secret_old"))
  if os.IsNotExist(err) {
    t.Errorf("Expected secret_old to be logged as expired but was not")
  } else if !bytes.Equal(savedBytes, []byte("234 567 abcd")) {
    t.Error("Expected secret_old expired record to contain the secret code \"234 567 abcd\", but contained", savedBytes)
  }

  if _, err = os.Stat(path.Join(s.BeingAccessedPath, "secret_being_accessed_not_old")); os.IsNotExist(err) {
    t.Error("Expected secret_being_accessed_not_old to exist but does not")
  }

  if _, err = os.Stat(path.Join(s.BeingAccessedPath, "secret_being_accessed_old")); !os.IsNotExist(err) {
    t.Error("Expected secret_being_accessed_old not to exist", err)
  }

  if _, err = os.Stat(path.Join(s.AccessedPath, "accessed_record_not_old")); os.IsNotExist(err) {
    t.Error("Expected accessed_record_not_old to exist but does not")
  }

  if _, err = os.Stat(path.Join(s.AccessedPath, "accessed_record_old")); !os.IsNotExist(err) {
    t.Error("Expected accessed_record_old not to exist but does", err)
  }

  if _, err = os.Stat(path.Join(s.ExpiringPath, "secret_expiring")); !os.IsNotExist(err) {
    t.Error("Expected secret_expiring not to exist but does", err)
  }

  if _, err = os.Stat(path.Join(s.ExpiredPath, "expired_record_not_old")); os.IsNotExist(err) {
    t.Error("Expected expired_record_not_old to exist but does not")
  }

  if _, err = os.Stat(path.Join(s.ExpiredPath, "expired_record_old")); !os.IsNotExist(err) {
    t.Error("Expected expired_record_old not to exist but does", err)
  }
}

func TestSweepSecrets(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  makeFile(s.Root, "not_old1", "234 567 abcd\n", 0, 0600)
  makeFile(s.Root, "not_old2", "234 567 abcd\nsecret", 9, 0600)
  makeFile(s.Root, "old1", "234 567 abcd\n", 10, 0600)
  makeFile(s.Root, "old2", "234 567 abcd\nsecret", 11, 0600)

  err := s.SweepSecrets()
  if err != nil {
    t.Error("Sweep secrets errored:", err)
  }

  // Removes secrets...

  files, err := ioutil.ReadDir(s.Root)
  if err != nil {
    t.Error("Error reading the secret store folder:", err)
    return
  }
  not_old1Found := false
  for _, fileInfo := range files {
    if "not_old1" == fileInfo.Name() {
      not_old1Found = true
    }
  }
  not_old2Found := false
  for _, fileInfo := range files {
    if "not_old2" == fileInfo.Name() {
      not_old2Found = true
    }
  }
  old1Found := false
  for _, fileInfo := range files {
    if "old1" == fileInfo.Name() {
      old1Found = true
    }
  }
  old2Found := false
  for _, fileInfo := range files {
    if "old2" == fileInfo.Name() {
      old2Found = true
    }
  }

  if !not_old1Found {
    t.Error("Expected to find secret not_old1 but did not!")
  }
  if !not_old2Found {
    t.Error("Expected to find secret not_old2 but did not!")
  }
  if old1Found {
    t.Error("Expected not to find secret old1 but did!")
  }
  if old2Found {
    t.Error("Expected not to find secret old2 but did!")
  }

  // Logs secrets as expired

  savedBytes, err := ioutil.ReadFile(path.Join(s.ExpiredPath, "old1"))
  if os.IsNotExist(err) {
    t.Errorf("Expected old1 to be logged as expired but was not")
  } else if !bytes.Equal(savedBytes, []byte("234 567 abcd")) {
    t.Error("Expected old1 expired record to contain the secret code \"234 567 abcd\", but contained", savedBytes)
  }

  savedBytes, err = ioutil.ReadFile(path.Join(s.ExpiredPath, "old2"))
  if os.IsNotExist(err) {
    t.Errorf("Expected old2 to be logged as expired but was not")
  } else if !bytes.Equal(savedBytes, []byte("234 567 abcd")) {
    t.Error("Expected old2 expired record to contain the secret code \"234 567 abcd\", but contained", savedBytes)
  }
}

func TestSweepBeingAccessed(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  ioutil.WriteFile(path.Join(s.BeingAccessedPath, "not_old1"), nil, 0600)
  ioutil.WriteFile(path.Join(s.BeingAccessedPath, "not_old2"), nil, 0600)
  ioutil.WriteFile(path.Join(s.BeingAccessedPath, "old1"), nil, 0600)
  ioutil.WriteFile(path.Join(s.BeingAccessedPath, "old2"), nil, 0600)

  os.Chtimes(path.Join(s.BeingAccessedPath, "not_old1"), time.Now(), time.Now().Add(-0*time.Minute))
  os.Chtimes(path.Join(s.BeingAccessedPath, "not_old2"), time.Now(), time.Now().Add(-10*time.Minute))
  os.Chtimes(path.Join(s.BeingAccessedPath, "old1"), time.Now(), time.Now().Add(-11*time.Minute))
  os.Chtimes(path.Join(s.BeingAccessedPath, "old2"), time.Now(), time.Now().Add(-12*time.Minute))

  err := s.SweepBeingAccessed()
  if err != nil {
    t.Error("Sweep being accessed errored:", err)
  }

  // Removes unlikely accidental leftovers in the being_accessed folder.
  // Assumes the accessed records have already been created.

  files, err := ioutil.ReadDir(s.BeingAccessedPath)
  if err != nil {
    t.Error("Error reading the being_accessed folder:", err)
    return
  }
  not_old1Found := false
  for _, fileInfo := range files {
    if "not_old1" == fileInfo.Name() {
      not_old1Found = true
    }
  }
  not_old2Found := false
  for _, fileInfo := range files {
    if "not_old2" == fileInfo.Name() {
      not_old2Found = true
    }
  }
  old1Found := false
  for _, fileInfo := range files {
    if "old1" == fileInfo.Name() {
      old1Found = true
    }
  }
  old2Found := false
  for _, fileInfo := range files {
    if "old2" == fileInfo.Name() {
      old2Found = true
    }
  }

  if !not_old1Found {
    t.Error("Expected to find secret not_old1 but did not!")
  }
  if !not_old2Found {
    t.Error("Expected to find secret not_old2 but did not!")
  }
  if old1Found {
    t.Error("Expected not to find secret old1 but did!")
  }
  if old2Found {
    t.Error("Expected not to find secret old2 but did!")
  }
}

func TestSweepExpiring(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  ioutil.WriteFile(path.Join(s.ExpiringPath, "not_old1"), nil, 0600)
  ioutil.WriteFile(path.Join(s.ExpiringPath, "not_old2"), nil, 0600)
  ioutil.WriteFile(path.Join(s.ExpiringPath, "old1"), nil, 0600)
  ioutil.WriteFile(path.Join(s.ExpiringPath, "old2"), nil, 0600)

  os.Chtimes(path.Join(s.ExpiringPath, "not_old1"), time.Now(), time.Now().Add(-0*time.Minute))
  os.Chtimes(path.Join(s.ExpiringPath, "not_old2"), time.Now(), time.Now().Add(-10*time.Minute))
  os.Chtimes(path.Join(s.ExpiringPath, "old1"), time.Now(), time.Now().Add(-11*time.Minute))
  os.Chtimes(path.Join(s.ExpiringPath, "old2"), time.Now(), time.Now().Add(-12*time.Minute))

  err := s.SweepExpiring()
  if err != nil {
    t.Error("Sweep expiring errored:", err)
  }

  // Removes expiring secrets...

  files, err := ioutil.ReadDir(s.ExpiringPath)
  if err != nil {
    t.Error("Error reading the expiring folder:", err)
    return
  }
  not_old1Found := false
  for _, fileInfo := range files {
    if "not_old1" == fileInfo.Name() {
      not_old1Found = true
    }
  }
  not_old2Found := false
  for _, fileInfo := range files {
    if "not_old2" == fileInfo.Name() {
      not_old2Found = true
    }
  }
  old1Found := false
  for _, fileInfo := range files {
    if "old1" == fileInfo.Name() {
      old1Found = true
    }
  }
  old2Found := false
  for _, fileInfo := range files {
    if "old2" == fileInfo.Name() {
      old2Found = true
    }
  }

  // Remove everything that ends up in the expiring folder, old or not.
  // (We move them to a folder because zeroing them in the main secrets
  // folder will change their modification time to be current, making
  // the the zeroed file available to the store's retreive functionality
  // for a fraction of a second before the file is removed.)
  if not_old1Found {
    t.Error("Expected not to find secret not_old1 but did!")
  }
  if not_old2Found {
    t.Error("Expected not to find secret not_old2 but did!")
  }
  if old1Found {
    t.Error("Expected not to find secret old1 but did!")
  }
  if old2Found {
    t.Error("Expected not to find secret old2 but did!")
  }
}

func TestSweepAccessed(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  ioutil.WriteFile(path.Join(s.AccessedPath, "not_old1"), nil, 0400)
  ioutil.WriteFile(path.Join(s.AccessedPath, "not_old2"), nil, 0400)
  ioutil.WriteFile(path.Join(s.AccessedPath, "old1"), nil, 0400)
  ioutil.WriteFile(path.Join(s.AccessedPath, "old2"), nil, 0400)

  os.Chtimes(path.Join(s.AccessedPath, "not_old1"), time.Now(), time.Now().Add(-0*time.Hour))
  os.Chtimes(path.Join(s.AccessedPath, "not_old2"), time.Now(), time.Now().Add(-23*time.Hour))
  os.Chtimes(path.Join(s.AccessedPath, "old1"), time.Now(), time.Now().Add(-24*time.Hour))
  os.Chtimes(path.Join(s.AccessedPath, "old2"), time.Now(), time.Now().Add(-25*time.Hour))

  err := s.SweepAccessed()
  if err != nil {
    t.Error("Sweep accessed errored:", err)
  }

  // Remove records of accessed secrets after 24 hours

  files, err := ioutil.ReadDir(s.AccessedPath)
  if err != nil {
    t.Error("Error reading the accessed folder:", err)
    return
  }
  not_old1Found := false
  for _, fileInfo := range files {
    if "not_old1" == fileInfo.Name() {
      not_old1Found = true
    }
  }
  not_old2Found := false
  for _, fileInfo := range files {
    if "not_old2" == fileInfo.Name() {
      not_old2Found = true
    }
  }
  old1Found := false
  for _, fileInfo := range files {
    if "old1" == fileInfo.Name() {
      old1Found = true
    }
  }
  old2Found := false
  for _, fileInfo := range files {
    if "old2" == fileInfo.Name() {
      old2Found = true
    }
  }

  if !not_old1Found {
    t.Error("Expected to find secret not_old1 but did not!")
  }
  if !not_old2Found {
    t.Error("Expected to find secret not_old2 but did not!")
  }
  if old1Found {
    t.Error("Expected not to find secret old1 but did!")
  }
  if old2Found {
    t.Error("Expected not to find secret old2 but did!")
  }
}

func TestSweepExpired(t *testing.T) {
  s := store.Setup()
  defer s.Teardown()

  ioutil.WriteFile(path.Join(s.ExpiredPath, "not_old1"), nil, 0400)
  ioutil.WriteFile(path.Join(s.ExpiredPath, "not_old2"), nil, 0400)
  ioutil.WriteFile(path.Join(s.ExpiredPath, "old1"), nil, 0400)
  ioutil.WriteFile(path.Join(s.ExpiredPath, "old2"), nil, 0400)

  os.Chtimes(path.Join(s.ExpiredPath, "not_old1"), time.Now(), time.Now().Add(-0*time.Hour))
  os.Chtimes(path.Join(s.ExpiredPath, "not_old2"), time.Now(), time.Now().Add(-23*time.Hour))
  os.Chtimes(path.Join(s.ExpiredPath, "old1"), time.Now(), time.Now().Add(-24*time.Hour))
  os.Chtimes(path.Join(s.ExpiredPath, "old2"), time.Now(), time.Now().Add(-25*time.Hour))

  err := s.SweepExpired()
  if err != nil {
    t.Error("Sweep expired errored:", err)
  }

  // Remove records of expired secrets after 24 hours

  files, err := ioutil.ReadDir(s.ExpiredPath)
  if err != nil {
    t.Error("Error reading the expired folder:", err)
    return
  }
  not_old1Found := false
  for _, fileInfo := range files {
    if "not_old1" == fileInfo.Name() {
      not_old1Found = true
    }
  }
  not_old2Found := false
  for _, fileInfo := range files {
    if "not_old2" == fileInfo.Name() {
      not_old2Found = true
    }
  }
  old1Found := false
  for _, fileInfo := range files {
    if "old1" == fileInfo.Name() {
      old1Found = true
    }
  }
  old2Found := false
  for _, fileInfo := range files {
    if "old2" == fileInfo.Name() {
      old2Found = true
    }
  }

  if !not_old1Found {
    t.Error("Expected to find secret not_old1 but did not!")
  }
  if !not_old2Found {
    t.Error("Expected to find secret not_old2 but did not!")
  }
  if old1Found {
    t.Error("Expected not to find secret old1 but did!")
  }
  if old2Found {
    t.Error("Expected not to find secret old2 but did!")
  }
}
