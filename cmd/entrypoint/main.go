package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	defaultCNIBinDir    = "/host/opt/cni/bin"
	defaultSRIOVBinFile = "/usr/bin/sriov"
)

func usage() {
	fmt.Fprintf(os.Stderr,
		"This is an entrypoint script for SR-IOV CNI to overlay its\n"+
			"binary into location in a filesystem. The binary file will\n"+
			"be copied to the corresponding directory.\n\n"+
			"./entrypoint\n"+
			"\t-h --help\n"+
			"\t--cni-bin-dir=%s\n"+
			"\t--sriov-bin-file=%s\n"+
			"\t--no-sleep\n",
		defaultCNIBinDir, defaultSRIOVBinFile)
}

func run() int {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cniBinDir := fs.String("cni-bin-dir", defaultCNIBinDir, "CNI binary destination directory")
	sriovBinFile := fs.String("sriov-bin-file", defaultSRIOVBinFile, "Source sriov binary path")
	noSleep := fs.Bool("no-sleep", false, "Exit after copying binary without waiting for signal")
	fs.Usage = usage

	err := fs.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to parse flags: %v\n", err)
		return 1
	}

	cniBinDirClean := filepath.Clean(*cniBinDir)
	if !filepath.IsAbs(cniBinDirClean) {
		fmt.Fprintf(os.Stderr, "cni-bin-dir must be an absolute path, got: %s\n", *cniBinDir)
		return 1
	}

	info, err := os.Stat(cniBinDirClean)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cni-bin-dir %q does not exist: %v\n", cniBinDirClean, err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "cni-bin-dir %q is not a directory\n", cniBinDirClean)
		return 1
	}

	if _, err := os.Stat(*sriovBinFile); err != nil {
		fmt.Fprintf(os.Stderr, "sriov-bin-file %q does not exist: %v\n", *sriovBinFile, err)
		return 1
	}

	binBase := filepath.Base(*sriovBinFile)
	destPath := filepath.Join(cniBinDirClean, binBase)
	tempPattern := fmt.Sprintf("%s.temp", binBase)
	if err := copyFileAtomic(*sriovBinFile, cniBinDirClean, tempPattern, binBase); err != nil {
		fmt.Fprintf(os.Stderr, "failed to copy %q to %q: %v\n", *sriovBinFile, destPath, err)
		return 1
	}

	if *noSleep {
		fmt.Println("SR-IOV CNI binary installed.")
		return 0
	}

	fmt.Println("SR-IOV CNI binary installed, waiting for termination signal.")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(ch)
	<-ch
	return 0
}

// copyFileAtomic does file copy atomically
func copyFileAtomic(srcFilePath, destDir, tempFileName, destFileName string) error {
	tempFilePath := filepath.Join(destDir, tempFileName)
	// check temp filepath and remove old file if exists
	if _, err := os.Stat(tempFilePath); err == nil {
		err = os.Remove(tempFilePath)
		if err != nil {
			return fmt.Errorf("cannot remove old temp file %q: %v", tempFilePath, err)
		}
	}

	// create temp file
	f, err := os.CreateTemp(destDir, tempFileName)
	if err != nil {
		return fmt.Errorf("cannot create temp file %q in %q: %v", tempFileName, destDir, err)
	}
	defer f.Close()

	srcFile, err := os.Open(srcFilePath)
	if err != nil {
		return fmt.Errorf("cannot open file %q: %v", srcFilePath, err)
	}
	defer srcFile.Close()

	// Copy file to tempfile
	_, err = io.Copy(f, srcFile)
	if err != nil {
		f.Close()
		os.Remove(tempFilePath)
		return fmt.Errorf("cannot write data to temp file %q: %v", tempFilePath, err)
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("cannot flush temp file %q: %v", tempFilePath, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("cannot close temp file %q: %v", tempFilePath, err)
	}

	// change file mode if different
	destFilePath := filepath.Join(destDir, destFileName)
	_, err = os.Stat(destFilePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	srcFileStat, err := os.Stat(srcFilePath)
	if err != nil {
		return err
	}

	if err := os.Chmod(f.Name(), srcFileStat.Mode()); err != nil {
		return fmt.Errorf("cannot set stat on temp file %q: %v", f.Name(), err)
	}

	// replace file with tempfile
	if err := os.Rename(f.Name(), destFilePath); err != nil {
		return fmt.Errorf("cannot replace %q with temp file %q: %v", destFilePath, tempFilePath, err)
	}

	return nil
}

func main() {
	os.Exit(run())
}
