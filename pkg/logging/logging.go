// package logging is a small wrapper around github.com/k8snetworkplumbingwg/cni-log

package logging

import (
	cnilog "github.com/k8snetworkplumbingwg/cni-log"
)

const (
	labelCNIName     = "cniName"
	labelContainerID = "containerID"
	labelNetNS       = "netns"
	labelIFName      = "ifname"
	cniName          = "sriov-cni"
)

var (
	logLevelDefault = cnilog.InfoLevel
	containerID     = ""
	netNS           = ""
	ifName          = ""
)

// Init initializes logging with the requested parameters in this order: log level, log file, container ID,
// network namespace and interface name.
func Init(logLevel, logFile, containerIdentification, networkNamespace, interfaceName string) {
	setLogLevel(logLevel)
	setLogFile(logFile)
	containerID = containerIdentification
	netNS = networkNamespace
	ifName = interfaceName
}

// setLogLevel sets the log level to either verbose, debug, info, warn, error or panic. If an invalid string is
// provided, it uses error.
func setLogLevel(l string) {
	ll := cnilog.StringToLevel(l)
	if ll == cnilog.InvalidLevel {
		ll = logLevelDefault
	}
	cnilog.SetLogLevel(ll)
}

// setLogFile sets the log file for logging. If the empty string is provided, it uses stderr.
func setLogFile(fileName string) {
	if fileName == "" {
		cnilog.SetLogStderr(true)
		cnilog.SetLogFile("")
		return
	}
	cnilog.SetLogFile(fileName)
	cnilog.SetLogStderr(false)
}

// Debug provides structured logging for log level >= debug.
func Debug(msg string, args ...interface{}) {
	cnilog.DebugStructured(msg, prependArgs(args)...)
}

// Info provides structured logging for log level >= info.
func Info(msg string, args ...interface{}) {
	cnilog.InfoStructured(msg, prependArgs(args)...)
}

// Warning provides structured logging for log level >= warning.
func Warning(msg string, args ...interface{}) {
	cnilog.WarningStructured(msg, prependArgs(args)...)
}

// Error provides structured logging for log level >= error.
func Error(msg string, args ...interface{}) {
	_ = cnilog.ErrorStructured(msg, prependArgs(args)...)
}

// Panic provides structured logging for log level >= panic.
func Panic(msg string, args ...interface{}) {
	cnilog.PanicStructured(msg, prependArgs(args)...)
}

// prependArgs prepends cniName, containerID, netNS and ifName to the args of every log message.
func prependArgs(args []interface{}) []interface{} {
	if ifName != "" {
		args = append([]interface{}{labelIFName, ifName}, args...)
	}
	if netNS != "" {
		args = append([]interface{}{labelNetNS, netNS}, args...)
	}
	if containerID != "" {
		args = append([]interface{}{labelContainerID, containerID}, args...)
	}
	args = append([]interface{}{labelCNIName, cniName}, args...)
	return args
}
