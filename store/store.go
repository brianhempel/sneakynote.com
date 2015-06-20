package store

import (
  "crypto/rand"
  "crypto/sha256"
  "encoding/hex"
  "errors"
  "fmt"
  "log"
  "io"
  "io/ioutil"
  "math/big"
  "os"
  "path"
  "strings"
  "time"
)

type Store struct {
  Root string
  BeingAccessedPath string
  AccessedPath string
  ExpiringPath string
  ExpiredPath string
  MaxSecretSize int
  Headroom int
  SecretLifetime time.Duration
}

const (
  CodeByteSize int = 12
  DefaultStorePath = "/tmp/sneakynote_store"
  DefaultMaxSecretSize int = 1024*16
  DefaultSecretLifetime time.Duration = 10*time.Minute
)

var (
  CodeAlphabet []string = []string{"2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f", "g", "h", "j", "k", "m", "n", "p", "q", "r", "s", "t", "v", "w", "x", "y", "z"}

  SecretTooLarge = errors.New("Secret too large")
  DuplicateId = errors.New("ID has been used before")
  StorageFull = errors.New("Secret storage full")

  SecretAlreadyAccessed = errors.New("Secret has already been accessed")
  SecretExpired = errors.New("Secret has expired without being accessed")
  SecretNotFound = errors.New("Secret not found")
)

func Get() *Store {
  storePath := DefaultStorePath
  beingAccessedPath := path.Join(storePath, "being_accessed")
  accessedPath := path.Join(storePath, "accessed")
  expiringPath := path.Join(storePath, "expiring")
  expiredPath := path.Join(storePath, "expired")
  maxSecretSize := DefaultMaxSecretSize

  return &Store{Root: storePath, BeingAccessedPath: beingAccessedPath, AccessedPath: accessedPath, ExpiringPath: expiringPath, ExpiredPath: expiredPath, MaxSecretSize: maxSecretSize, Headroom: DefaultHeadroom, SecretLifetime: DefaultSecretLifetime}
}

func Setup() *Store {
  s := Get()

  err := setupRamDisk(s.Root)
  if err != nil {
    log.Fatal("Creating ramdisk: ", err)
  }

  if _, err := os.Stat(s.BeingAccessedPath); os.IsNotExist(err) {
    err = os.Mkdir(s.BeingAccessedPath, 0700)
    if err != nil {
      log.Fatal("Making being_accessed dir for store: ", err)
    }
  }

  if _, err := os.Stat(s.AccessedPath); os.IsNotExist(err) {
    err = os.Mkdir(s.AccessedPath, 0700)
    if err != nil {
      log.Fatal("Making accessed dir for store: ", err)
    }
  }

  if _, err := os.Stat(s.ExpiredPath); os.IsNotExist(err) {
    err = os.Mkdir(s.ExpiringPath, 0700)
    if err != nil {
      log.Fatal("Making expiring dir for store: ", err)
    }
  }

  if _, err := os.Stat(s.ExpiredPath); os.IsNotExist(err) {
    err = os.Mkdir(s.ExpiredPath, 0700)
    if err != nil {
      log.Fatal("Making expired dir for store: ", err)
    }
  }

  return s
}

func (s *Store) AvailableMemory() int {
  freeBytes, err := s.freeSpace()
  if err != nil {
    log.Print("Error determining free space", err)
    return -1
  }

  return freeBytes - s.Headroom
}

func (s *Store) Save(data io.Reader, uuid string) (string, error) {
  fileName := s.UuidToFileName(uuid)
  filePath := s.uuidToFilePath(uuid)

  err := s.accessedOrExpired(fileName)
  if err != nil {
    return "", DuplicateId
  }

  buf := make([]byte, s.maxSecretStorageSize() + 1)
  code, err := generateCode()
  if err != nil {
    log.Print("Error generating code:", err)
    return "", err
  }
  codePart := []byte(code + "\n")
  copy(buf[:len(codePart)], codePart)

  nRead, err := io.ReadFull(data, buf[len(codePart):])
  // Zero out our buffer when done
  defer func() {
    for i := 0; i < len(buf); i++ {
      buf[i] = 0
    }
  }()

  if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
    log.Print("Error reading request body:", err)
    return "", err
  } else {
    err = nil
  }

  if nRead > s.MaxSecretSize {
    return "", SecretTooLarge
  }

  available := s.AvailableMemory()
  if nRead > available {
    return "", StorageFull
  } else if available < 0 {
    return "", errors.New("Could not determine storage free space")
  }

  // Same secret sent twice? Kill the secret to penalize the client or thwart
  // the attacker trying to replace the secret.
  if _, err := os.Stat(filePath); !os.IsNotExist(err) {
    beingAccessedFilePath := path.Join(s.BeingAccessedPath, fileName)

    err = os.Rename(filePath, beingAccessedFilePath)
    defer zeroFileAndRemove(beingAccessedFilePath)

    accessedFilePath := path.Join(s.AccessedPath, fileName)
    ioutil.WriteFile(accessedFilePath, nil, 0600)

    // Extract the code

    beingAccessedFile, err := os.Open(beingAccessedFilePath)
    if err != nil {
      log.Print("Error opening file:", err)
      return "", DuplicateId
    }

    codePart := make([]byte, CodeByteSize + 1) // Grab newline.
    _, err = io.ReadFull(beingAccessedFile, codePart)
    if err != nil {
      log.Print("Error reading code from file:", err)
    } else {
      // Remove newline.
      code := strings.TrimSpace(string(codePart))

      // Update the accessed file with the code

      ioutil.WriteFile(accessedFilePath, []byte(code), 0400)
    }

    return "", DuplicateId
  }

  err = ioutil.WriteFile(filePath, buf[:(len(codePart)+nRead)], 0600)
  if err != nil && strings.Contains(err.Error(), "no space left on device") {
    return "", StorageFull
  } else if err != nil {
    log.Print("Error writing secret file:", err)
    return "", err
  }

  // Attempt to clear the secret out of memory.
  // Zero out the request buffer
  // for i := 0; i < len(data.buf); i++ {
  //   data.buf[i] = 0
  // }

  // if bufioReadWriter, ok := data.(bufio.ReadWriter)
  // a := bufioReadWriter
  // bufioReadWriter = a

  // I got this far in reading server.go:
  // func (srv *Server) newConn(rwc net.Conn) (c *conn, err error) {
  //   c = new(conn)
  //   c.remoteAddr = rwc.RemoteAddr().String()
  //   c.server = srv
  //   c.rwc = rwc
  //   c.w = rwc
  //   if debugServerConnections {
  //     c.rwc = newLoggingConn("server", c.rwc)
  //   }
  //   c.sr = liveSwitchReader{r: c.rwc}
  //   c.lr = io.LimitReader(&c.sr, noLimit).(*io.LimitedReader)
  //   br := newBufioReader(c.lr)
  //   bw := newBufioWriterSize(checkConnErrorWriter{c}, 4<<10)
  //   c.buf = bufio.NewReadWriter(br, bw)
  //   return c, nil
  // }

  // The out.Write call hands the slice down to
  // the write syscall with no other go buffers, so
  // nothing to do there. Well, maybe there is, but
  // I'm not smart enough right now to know.

  return code, err
}

// returns nRead, code, err
func (s *Store) Retrieve(id string, buf []byte) (int, string, error) {
  fileName := s.UuidToFileName(id)
  filePath := s.uuidToFilePath(id)

  tempRand := make([]byte, 32)
  _, err := rand.Read(tempRand)
  if err != nil {
    log.Print("Error getting random bytes:", err)
    return -1, "", err
  }
  tempFileName := hex.EncodeToString(tempRand)
  tempFilePath := path.Join(s.BeingAccessedPath, tempFileName)

  fileInfo, err := os.Stat(filePath)
  if err == nil {
    cutoff := time.Now().Add(-s.SecretLifetime)
    if fileInfo.ModTime().Before(cutoff) {
      return -1, "", SecretExpired
    }
  }

  err = os.Rename(filePath, tempFilePath)
  defer zeroFileAndRemove(tempFilePath)

  if err != nil {
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
      return -1, "", s.retrieveNotFoundError(fileName)
    }
    log.Print("Error moving file:", err)
    return -1, "", err
  }

  // Make a record of this access ASAP

  accessedFilePath := path.Join(s.AccessedPath, fileName)
  ioutil.WriteFile(accessedFilePath, nil, 0600)

  tempFile, err := os.Open(tempFilePath)
  defer tempFile.Close()
  if err != nil {
    log.Print("Error opening file:", err)
    return -1, "", err
  }

  // Extract the code

  codePart := make([]byte, CodeByteSize + 1) // Grab newline.
  _, err = io.ReadFull(tempFile, codePart)
  if err != nil {
    log.Print("Error reading code from file:", err)
    return -1, "", err
  }
  // Remove newline.
  code := strings.TrimSpace(string(codePart))

  // Update the accessed record with the code

  ioutil.WriteFile(accessedFilePath, []byte(code), 0400)

  // Read the secret

  nRead, err := io.ReadFull(tempFile, buf)
  if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
    log.Print("Error reading file:", err)
    return -1, "", err
  }

  return nRead, code, nil
}

func (s *Store) Status(id string, givenCode string) (error) {
  fileName := s.UuidToFileName(id)

  err, secretCode := s.locateSecretAndCode(fileName)

  if givenCode != "" && givenCode == secretCode {
    return err
  } else {
    return SecretNotFound
  }
}

func (s *Store) locateSecretAndCode(fileName string) (error, string) {
  accessedFilePath := path.Join(s.AccessedPath, fileName)
  expiredFilePath := path.Join(s.ExpiredPath, fileName)
  secretFilePath := path.Join(s.Root, fileName)

  // The secret file in particular could be moved/destroyed between the time
  // we determine its existance and the time we try to read the secret from it.
  //
  // Therfore, watch for errors and retry.

  for retries := 0; retries < 3; retries++ {

    if code, err := readCode(accessedFilePath); err == nil {

      return SecretAlreadyAccessed, code

    } else if code, err := readCode(expiredFilePath); err == nil {

      return SecretExpired, code

    } else if code, err := readCode(secretFilePath); err == nil {

      fileInfo, err := os.Stat(secretFilePath)
      if err == nil {
        cutoff := time.Now().Add(-s.SecretLifetime)
        if fileInfo.ModTime().Before(cutoff) {
          return SecretExpired, code
        } else {
          return nil, code
        }
      }

    }

    time.Sleep(time.Millisecond * 50)
  }

  return SecretNotFound, ""
}

// Read only the code from a secret file, accessed record, or expired record
func readCode(path string) (string, error) {
  code := make([]byte, CodeByteSize)

  file, err := os.Open(path)
  defer file.Close()
  if err != nil { return "", err }

  _, err = io.ReadFull(file, code)
  if err != nil { return "", err }

  return string(code), nil
}

func (s *Store) retrieveNotFoundError(fileName string) error {
  err := s.accessedOrExpired(fileName)

  if err != nil {
    return err
  }

  return SecretNotFound
}

func (s *Store) accessedOrExpired(fileName string) error {
  accessedFilePath := path.Join(s.AccessedPath, fileName)
  expiredFilePath := path.Join(s.ExpiredPath, fileName)

  if _, err := os.Stat(accessedFilePath); !os.IsNotExist(err) {
    return SecretAlreadyAccessed
  } else if _, err := os.Stat(expiredFilePath); !os.IsNotExist(err) {
    return SecretExpired
  }

  return nil
}

func GenerateUuid() (string) {
  var uuid string;
  bytes := make([]byte, 16)
  _, err := rand.Read(bytes)
  if err != nil {
    log.Print("Error getting random bytes:", err)
    return ""
  }
  bytes[6] = bytes[6] & 0x0f | 0x40
  bytes[8] = bytes[8] & 0x3f | 0x80

  uuid = fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])

  // log.Printf(uuid)
  // decoded, _ := hex.DecodeString(strings.Replace(uuid, "-", "", -1))
  // log.Printf("%x", decoded)

  return uuid
}

func (s *Store) UuidToFileName(uuid string) (string) {
  idBytes, err := hex.DecodeString(strings.Replace(uuid, "-", "", -1))
  if err != nil {
    log.Print("Error converting uuid to bytes:", err, " ", uuid)
    return ""
  }
  hashed := sha256.Sum256(idBytes)
  fileName := hex.EncodeToString(hashed[:])

  return fileName
}

func (s *Store) uuidToFilePath(uuid string) (string) {
  fileName := s.UuidToFileName(uuid)
  if fileName != "" {
    return path.Join(s.Root, fileName)
  } else {
    return ""
  }
}

func (s *Store) maxSecretStorageSize() (int) {
  return s.MaxSecretSize + CodeByteSize + 1
}

func generateCodeChar() (string, error) {
  maxI := big.NewInt(int64(len(CodeAlphabet)))

  i, err := rand.Int(rand.Reader, maxI)

  return CodeAlphabet[i.Int64()], err
}

func generateCode() (string, error) {
  c1, err := generateCodeChar()
  if err != nil { return "", err }
  c2, err := generateCodeChar()
  if err != nil { return "", err }
  c3, err := generateCodeChar()
  if err != nil { return "", err }
  c4, err := generateCodeChar()
  if err != nil { return "", err }
  c5, err := generateCodeChar()
  if err != nil { return "", err }
  c6, err := generateCodeChar()
  if err != nil { return "", err }
  c7, err := generateCodeChar()
  if err != nil { return "", err }
  c8, err := generateCodeChar()
  if err != nil { return "", err }
  c9, err := generateCodeChar()
  if err != nil { return "", err }
  c10, err := generateCodeChar()
  if err != nil { return "", err }

  return c1+c2+c3+" "+c4+c5+c6+" "+c7+c8+c9+c10, nil
}

func zeroFileAndRemove(filePath string) error {
  defer os.Remove(filePath)

  file, err := os.OpenFile(filePath, os.O_RDWR, 0600)
  if err != nil {
    file.Close()
    return err
  }

  fileInfo, err := file.Stat()
  if err != nil {
    file.Close()
    return err
  }

  _, err = file.Seek(0, os.SEEK_SET)
  if err != nil {
    file.Close()
    return err
  }

  zeros := make([]byte, fileInfo.Size())
  _, err = file.Write(zeros)
  if err != nil {
    file.Close()
    return err
  }

  err = file.Sync()
  if err != nil {
    file.Close()
    return err
  }

  err = file.Close()
  if err != nil {
    return err
  }

  return nil
}
