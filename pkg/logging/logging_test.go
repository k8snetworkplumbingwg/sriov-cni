package logging

import (
	"fmt"
	"io"
	"os"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

var _ = g.Describe("Logging", func() {
	var origStderr *os.File
	var stderrFile *os.File

	g.BeforeEach(func() {
		var err error
		stderrFile, err = os.CreateTemp("", "")
		o.Expect(err).NotTo(o.HaveOccurred())
		origStderr = os.Stderr
		os.Stderr = stderrFile
	})

	g.AfterEach(func() {
		os.Stderr = origStderr
		o.Expect(stderrFile.Close()).To(o.Succeed())
		o.Expect(os.RemoveAll(stderrFile.Name())).To(o.Succeed())
	})

	g.Context("log argument prepender", func() {
		g.When("none of netns, containerID, ifName are specified", func() {
			g.BeforeEach(func() {
				Init("", "", "", "", "")
			})

			g.It("should only prepend the cniName", func() {
				Panic("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				//nolint:gocritic
				o.Expect(out).Should(o.ContainSubstring(fmt.Sprintf(`%s="%s"`, labelCNIName, cniName)))
				o.Expect(out).ShouldNot(o.ContainSubstring(labelContainerID))
				o.Expect(out).ShouldNot(o.ContainSubstring(labelNetNS))
				o.Expect(out).ShouldNot(o.ContainSubstring(labelIFName))
			})
		})

		g.When("netns, containerID and ifName are specified", func() {
			const (
				testContainerID = "test-containerid"
				testNetNS       = "test-netns"
				testIFName      = "test-ifname"
			)

			g.BeforeEach(func() {
				Init("", "", testContainerID, testNetNS, testIFName)
			})

			g.It("should log cniName, netns, containerID and ifName", func() {
				Panic("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				//nolint:gocritic
				o.Expect(out).Should(o.ContainSubstring(fmt.Sprintf(`%s="%s"`, labelCNIName, cniName)))
				//nolint:gocritic
				o.Expect(out).Should(o.ContainSubstring(fmt.Sprintf(`%s="%s"`, labelContainerID, testContainerID)))
				//nolint:gocritic
				o.Expect(out).Should(o.ContainSubstring(fmt.Sprintf(`%s="%s"`, labelNetNS, testNetNS)))
				//nolint:gocritic
				o.Expect(out).Should(o.ContainSubstring(fmt.Sprintf(`%s="%s"`, labelIFName, testIFName)))
			})
		})
	})

	g.Context("log levels", func() {
		g.When("the defaults are used", func() {
			g.BeforeEach(func() {
				Init("", "", "", "", "")
			})

			g.It("panic messages are logged to stderr", func() {
				Panic("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("test message"))
			})

			g.It("info messages are logged to stderr and look as expected", func() {
				Info("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring(`msg="test message"`))
				o.Expect(out).Should(o.ContainSubstring(`a="b"`))
				o.Expect(out).Should(o.ContainSubstring(`level="info"`))
			})

			g.It("debug messages are not logged to stderr", func() {
				Debug("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).ShouldNot(o.ContainSubstring("test message"))
			})
		})

		g.When("the log level is raised to warning", func() {
			g.BeforeEach(func() {
				Init("warning", "", "", "", "")
			})

			g.It("panic messages are logged to stderr", func() {
				Panic("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("test message"))
			})

			g.It("error messages are logged to stderr", func() {
				Error("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("test message"))
			})

			g.It("warning messages are logged to stderr", func() {
				Warning("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("test message"))
			})

			g.It("info messages are not logged to stderr", func() {
				Info("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).ShouldNot(o.ContainSubstring("test message"))
			})
		})

		g.When("the log level is set to an invalid value", func() {
			g.BeforeEach(func() {
				Init("I'm invalid", "", "", "", "")
			})

			g.It("panic messages are logged to stderr", func() {
				Panic("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("test message"))
			})

			g.It("info messages are logged to stderr", func() {
				Info("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("test message"))
			})

			g.It("debug messages are not logged to stderr", func() {
				Debug("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).ShouldNot(o.ContainSubstring("test message"))
			})
		})
	})

	g.Context("log files", func() {
		var logFile *os.File

		g.BeforeEach(func() {
			var err error
			logFile, err = os.CreateTemp("", "")
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			o.Expect(logFile.Close()).To(o.Succeed())
			o.Expect(os.RemoveAll(logFile.Name())).To(o.Succeed())
		})

		g.When("the log file is set", func() {
			g.BeforeEach(func() {
				Init("", logFile.Name(), "", "", "")
			})

			g.It("error messages are logged to log file but not to stderr", func() {
				Error("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(logFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("test message"))

				_, _ = stderrFile.Seek(0, 0)
				out, err = io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).ShouldNot(o.ContainSubstring("test message"))
			})
		})

		g.When("the log file is set and then unset", func() {
			g.BeforeEach(func() {
				// TODO: This triggers a data race in github.com/k8snetworkplumbingwg/cni-log; fix the datarace in the
				// logging component and then remove the skip.
				g.Skip("https://github.com/k8snetworkplumbingwg/cni-log/issues/15")
				Init("", logFile.Name(), "", "", "")
				setLogFile("")
			})

			g.It("logs to stderr but not to file", func() {
				Error("test message", "a", "b")
				_, _ = stderrFile.Seek(0, 0)
				out, err := io.ReadAll(logFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).ShouldNot(o.ContainSubstring("test message"))

				_, _ = stderrFile.Seek(0, 0)
				out, err = io.ReadAll(stderrFile)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring("test message"))
			})
		})
	})
})
