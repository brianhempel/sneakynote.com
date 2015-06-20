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
  DefaultHeadroom int = (DefaultMaxSecretSize + CodeByteSize + 1) * 3
)

func setupRamDisk(path string) (error) {
  err := exec.Command("umount", "-f", path).Run()
  if err == nil {
    log.Printf("Unmounted ramdisk at %s - you may want to eject it!", path)
  }

  // 1 MB
  diskPath, err := exec.Command("hdiutil", "attach", "-nomount", "ram://2048").Output()
  if err != nil {
    log.Fatal("Creating ramdisk: ", err)
  }
  diskPathStr := strings.TrimSpace(string(diskPath))
  log.Printf("Created ramdisk at %s", diskPathStr)

  err = exec.Command("newfs_hfs", diskPathStr).Run()
  if err != nil {
    log.Fatal("Formatting ramdisk: ", err)
  }
  log.Printf("Formatted ramdisk as HFS.")

  if _, err := os.Stat(path); os.IsNotExist(err) {
    err = os.Mkdir(path, 0700)
    if err != nil {
      log.Fatal("Making dir for ramdisk: ", err)
    }
  }

  err = exec.Command("mount", "-t", "hfs", diskPathStr, path).Run()
  if err != nil {
    log.Fatal("Mounting ramdisk: ", err)
  }
  log.Printf("Ramdisk mounted at %s", path)

  return nil
}

func (s *Store) Teardown() error {
  out, err := exec.Command("hdiutil", "detach", "-force", s.Root).CombinedOutput()
  if err != nil {
    // Sometimes there's a resource-busy error...sleep and retry
    time.Sleep(time.Second)
    out, err = exec.Command("hdiutil", "detach", "-force", s.Root).CombinedOutput()
  }
  if err != nil {
    log.Print("Umounting/ejecting ramdisk: ", err, " ", string(out))
    return err
  }
  log.Printf("Ramdisk %s unmounted and ejected.", s.Root)

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
  df, err := exec.Command("df", s.Root).CombinedOutput()
  if err != nil {
    log.Print("Error getting store free space: ", err, string(df))
    return -1, err
  }
  // log.Print(string(df))
  // Filesystem 512-blocks Used Available Capacity iused ifree %iused  Mounted on
  // /dev/disk3       2048  288      1760    15%      34   220   13%   /private/tmp/hello

  regexp, err := regexp.Compile("\\s\\d+\\s")
  if err != nil {
    log.Print("Error compiling regexp: ", err)
    return -1, err
  }
  matches := regexp.FindAll(df, -1)
  if matches == nil {
    log.Print("Error matching df output")
    return -1, err
  }

  freeBlocksStr := strings.TrimSpace(string(matches[2]))
  freeBlocks, err := strconv.ParseInt(freeBlocksStr, 10, 64)
  if err != nil {
    log.Print("Error parsing freeBlocks string: ", err)
    return -1, err
  }
  // log.Print(free * 512)

  return int(freeBlocks) * 512, nil
}
