package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// writeTempFile creates a temporary file with the given content and permissions.
func writeTempFile(dir, name string, content []byte, perm os.FileMode) string {
	path := filepath.Join(dir, name)
	Expect(os.WriteFile(path, content, perm)).To(Succeed())
	return path
}

var _ = Describe("copyFile", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "entrypoint-test-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, tmpDir)
	})

	It("copies file content to destination", func() {
		content := []byte("binary-content")
		src := writeTempFile(tmpDir, "src", content, 0o644)
		dst := filepath.Join(tmpDir, "dst")

		Expect(copyFileAtomic(src, tmpDir, "src.temp", "dst")).To(Succeed())

		got, err := os.ReadFile(dst)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(content))
	})

	It("preserves source file permissions on the destination", func() {
		src := writeTempFile(tmpDir, "src", []byte("data"), 0o755)
		dst := filepath.Join(tmpDir, "dst")

		Expect(copyFileAtomic(src, tmpDir, "src.temp", "dst")).To(Succeed())

		info, err := os.Stat(dst)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o755)))
	})

	It("overwrites an existing destination file", func() {
		src := writeTempFile(tmpDir, "src", []byte("new-content"), 0o755)
		dst := writeTempFile(tmpDir, "dst", []byte("old-content"), 0o644)

		Expect(copyFileAtomic(src, tmpDir, "src.temp", "dst")).To(Succeed())

		got, err := os.ReadFile(dst)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal([]byte("new-content")))
	})

	It("returns an error when the source does not exist", func() {
		err := copyFileAtomic(filepath.Join(tmpDir, "nonexistent"), tmpDir, "src.temp", "dst")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error when the destination directory does not exist", func() {
		src := writeTempFile(tmpDir, "src", []byte("data"), 0o755)
		err := copyFileAtomic(src, filepath.Join(tmpDir, "no-such-dir"), "src.temp", "dst")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("run", func() {
	var (
		origArgs []string
		tmpDir   string
	)

	BeforeEach(func() {
		origArgs = os.Args
		var err error
		tmpDir, err = os.MkdirTemp("", "entrypoint-run-test-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			os.Args = origArgs
			os.RemoveAll(tmpDir)
		})
	})

	It("returns 1 when --cni-bin-dir is a relative path", func() {
		src := writeTempFile(tmpDir, "sriov", []byte("bin"), 0o755)
		os.Args = []string{"entrypoint", "--cni-bin-dir=relative/path", "--sriov-bin-file=" + src}
		Expect(run()).To(Equal(1))
	})

	It("returns 1 when --cni-bin-dir does not exist", func() {
		src := writeTempFile(tmpDir, "sriov", []byte("bin"), 0o755)
		os.Args = []string{"entrypoint",
			"--cni-bin-dir=" + filepath.Join(tmpDir, "no-such-dir"),
			"--sriov-bin-file=" + src,
		}
		Expect(run()).To(Equal(1))
	})

	It("returns 1 when --cni-bin-dir points to a file, not a directory", func() {
		notADir := writeTempFile(tmpDir, "notadir", []byte("x"), 0o644)
		src := writeTempFile(tmpDir, "sriov", []byte("bin"), 0o755)
		os.Args = []string{"entrypoint",
			"--cni-bin-dir=" + notADir,
			"--sriov-bin-file=" + src,
		}
		Expect(run()).To(Equal(1))
	})

	It("returns 1 when --sriov-bin-file does not exist", func() {
		os.Args = []string{"entrypoint",
			"--cni-bin-dir=" + tmpDir,
			"--sriov-bin-file=" + filepath.Join(tmpDir, "no-such-binary"),
		}
		Expect(run()).To(Equal(1))
	})

	It("copies the binary to --cni-bin-dir and exits 0 on SIGTERM", func() {
		binContent := []byte("#!/bin/sh\necho hello")
		src := writeTempFile(tmpDir, "sriov", binContent, 0o755)
		destDir, err := os.MkdirTemp("", "entrypoint-dest-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, destDir)

		entrypointBin := filepath.Join(tmpDir, "entrypoint")
		buildOut, err := exec.Command("go", "build", "-o", entrypointBin, ".").CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "go build failed: %s", buildOut)

		proc := exec.Command(entrypointBin,
			"--cni-bin-dir="+destDir,
			"--sriov-bin-file="+src,
		)
		proc.Stdout = GinkgoWriter
		proc.Stderr = GinkgoWriter
		Expect(proc.Start()).To(Succeed())
		DeferCleanup(func() { _ = proc.Process.Kill() })

		destFile := filepath.Join(destDir, "sriov")
		Eventually(destFile, 5*time.Second, 50*time.Millisecond).Should(BeAnExistingFile())

		got, err := os.ReadFile(destFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(binContent))
		info, err := os.Stat(destFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o755)))

		Expect(proc.Process.Signal(syscall.SIGTERM)).To(Succeed())
		Expect(proc.Wait()).To(Succeed())
		Expect(proc.ProcessState.ExitCode()).To(Equal(0))
	})
})
