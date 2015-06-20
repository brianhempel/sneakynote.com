// Mac OS X specific storage logic.

package store

import (
  "log"
  "os"
  "os/exec"
  "regexp"
  "strconv"
  "strings"
  "time"
)

const (
  DefaultHeadroom int = 1024*1024*30
)

var (
  freeSpaceRegexp = regexp.MustCompile("\\s\\d+\\s")
)

func setupRamDisk(path string) (string, error) {
  err := exec.Command("sudo", "umount", path).Run()
  if err == nil {
    log.Printf("Unmounted ramdisk at %s!", path)
  }

  err = os.MkdirAll(path, 0700)
  if err != nil {
    log.Fatal("Creating ramdisk folder: ", err)
  }

  err = exec.Command("sudo", "mount", "-t", "ramfs", "-o", "size=1m", "ramfs", path).Run()
  if err != nil {
    log.Fatal("Mounting ramdisk: ", err)
  }
  log.Printf("Ramdisk mounted at %s", path)

  return path, nil
}

func (s *Store) Teardown() error {
  out, err := exec.Command("sudo", "umount", s.DiskPath).CombinedOutput()
  if err != nil {
    // Sometimes there's a resource-busy error...sleep and retry
    time.Sleep(time.Second)
    out, err = exec.Command("sudo", "umount", s.DiskPath).CombinedOutput()
  }
  if err != nil {
    log.Print("Umounting/ejecting ramdisk: ", err, " ", string(out))
    return err
  }
  log.Printf("Ramdisk %s unmounted and ejected.", s.DiskPath)

  // rm -r is dangerous...
  if isMatch, _ := regexp.MatchString("\\A/tmp/[^/]+", s.Root); isMatch {
    out, err = exec.Command("rm", "-r", s.Root).CombinedOutput()
    // if err != nil {
    //   // Sometimes there's an error
    //   time.Sleep(time.Second)
    //   err = exec.Command("rm", "-r", s.Root).Run()
      if err != nil {
        log.Print("rm -r: ", err, " ", string(out))
        return err
      }
    // }
    log.Printf("Mountpoint folder %s removed.", s.Root)
  }

  return nil
}

func (s *Store) freeSpace() (int, error) {
  // On Linux with ramfs, just use free system memory minus a threshold
  // On Mac OS X we can read straigh out of the store
  out, err := exec.Command("free", "-b").CombinedOutput()
  if err != nil {
    log.Print("Error getting computer free space: ", err, string(out))
    return -1, err
  }
  // log.Print(string(df))
  //             total       used       free     shared    buffers     cached
  // Mem:     249675776   72126464  177549312     360448    4898816   14766080
  // -/+ buffers/cache:   52461568  197214208
  // Swap:            0          0          0

  matches := freeSpaceRegexp.FindAll(out, -1)
  if matches == nil {
    log.Print("Error matching free output")
    return -1, err
  }

  freeBytesStr := strings.TrimSpace(string(matches[2]))
  freeBytes, err := strconv.ParseInt(freeBytesStr, 10, 64)
  if err != nil {
    log.Print("Error parsing freeBytes string: ", err)
    return -1, err
  }

  return int(freeBytes), nil
}
