package store

import (
  "io/ioutil"
  "log"
  "os"
  "path"
  "time"
)

func (s *Store) SweepContinuously() {
  for {
    s.Sweep()
    time.Sleep(time.Minute)
  }
}

func (s *Store) Sweep() error {
  err := s.SweepSecrets()
  if err != nil {
    return err
  }

  err = s.SweepBeingAccessed()
  if err != nil {
    return err
  }

  err = s.SweepAccessed()
  if err != nil {
    return err
  }

  err = s.SweepExpiring()
  if err != nil {
    return err
  }

  err = s.SweepExpired()
  if err != nil {
    return err
  }

  return nil
}

func (s *Store) SweepSecrets() error {
  files, err := ioutil.ReadDir(s.Root)

  if err != nil {
    log.Print("Error reading store to sweep secrets:", err)
    return err
  }

  cutoff := time.Now().Add(-s.SecretLifetime)

  for _, fileInfo := range files {
    if !fileInfo.IsDir() && fileInfo.ModTime().Before(cutoff) {
      // Make a record of this secret's expiration
      expiredFilePath := path.Join(s.ExpiredPath, fileInfo.Name())
      ioutil.WriteFile(expiredFilePath, nil, 0600)

      // Move secret to expiring folder. Sweep there will zero and remove it.
      expiringFilePath := path.Join(s.ExpiringPath, fileInfo.Name())
      filePath := path.Join(s.Root, fileInfo.Name())

      err = os.Rename(filePath, expiringFilePath)

      if err != nil {
        // Just log the error, don't abort sweep.
        log.Print("Error moving", filePath, "to", expiringFilePath, err)
      } else {
        // Rewrite the expired record with the code from the secret file
        code := make([]byte, CodeByteSize)
        file, err := os.Open(expiringFilePath)
        if err != nil {
          log.Print("Error opening", filePath, err)
        } else {
          _, err := file.Read(code)
          if err != nil {
            log.Print("Error reading", filePath, err)
          }
          file.Close()
          ioutil.WriteFile(expiredFilePath, code, 0400)
        }
      }
    }
  }

  return nil
}

// This folder shouldn't have leftovers, but just in case...
func (s *Store) SweepBeingAccessed() error {
  // One minute to read the secret should be more than plenty
  return sweepFolder(s.BeingAccessedPath, s.SecretLifetime + time.Minute)
}

func (s *Store) SweepAccessed() error {
  return sweepFolder(s.AccessedPath, 24 * time.Hour)
}

func (s *Store) SweepExpiring() error {
  return sweepFolder(s.ExpiringPath, 0)
}

func (s *Store) SweepExpired() error {
  return sweepFolder(s.ExpiredPath, 24 * time.Hour)
}

func sweepFolder(folderPath string, maxAge time.Duration) error {
  files, err := ioutil.ReadDir(folderPath)
  if err != nil {
    log.Print("Error reading", folderPath, "folder to sweep secrets:", err)
    return err
  }

  cutoff := time.Now().Add(-maxAge)

  for _, fileInfo := range files {
    if !fileInfo.IsDir() && fileInfo.ModTime().Before(cutoff) {
      filePath := path.Join(folderPath, fileInfo.Name())
      zeroFileAndRemove(filePath)
    }
  }

  return nil
}