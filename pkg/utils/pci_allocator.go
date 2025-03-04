package utils

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/logging"

	"golang.org/x/sys/unix"
)

const pciLockAcquireTimeout = 60 * time.Second

type PCIAllocation interface {
	SaveAllocatedPCI(string, string) error
	DeleteAllocatedPCI(string) error
	IsAllocated(string) error
}

type PCIAllocator struct {
	dataDir string
}

// NewPCIAllocator returns a new PCI allocator
// it will use the <dataDir>/pci folder to store the information about allocated PCI addresses
func NewPCIAllocator(dataDir string) *PCIAllocator {
	return &PCIAllocator{dataDir: filepath.Join(dataDir, "pci")}
}

// Lock gets an exclusive lock on the given PCI address, ensuring there is no other process configuring / or de-configuring the same device.
func (p *PCIAllocator) Lock(pciAddress string) error {
	lockDir := path.Join(p.dataDir, "vf_lock")
	if err := os.MkdirAll(lockDir, 0600); err != nil {
		return fmt.Errorf("failed to create the sriov lock directory(%q): %v", lockDir, err)
	}

	path := filepath.Join(lockDir, fmt.Sprintf("%s.lock", pciAddress))

	// unix.O_CREAT - Create the file if it doesn't exist
	// unix.O_RDONLY - Open the file for read
	// unix.O_CLOEXEC - Automatically close the file on exit. This is useful to keep the flock until the process exits
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDONLY|unix.O_CLOEXEC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open PCI file [%s] for locking: %w", path, err)
	}

	errCh := make(chan error)
	go func() {
		// unix.LOCK_EX - Exclusive lock
		errCh <- unix.Flock(fd, unix.LOCK_EX)
	}()

	select {
	case err = <-errCh:
		if err != nil {
			return fmt.Errorf("failed to flock PCI file [%s]: %w", path, err)
		}
		return nil

	case <-time.After(pciLockAcquireTimeout):
		return fmt.Errorf("time out while waiting to acquire exclusive lock on [%s]", path)
	}
}

// SaveAllocatedPCI creates a file with the pci address as a name and the network namespace as the content
// return error if the file was not created
func (p *PCIAllocator) SaveAllocatedPCI(pciAddress, ns string) error {
	if err := os.MkdirAll(p.dataDir, 0600); err != nil {
		return fmt.Errorf("failed to create the sriov data directory(%q): %v", p.dataDir, err)
	}

	path := filepath.Join(p.dataDir, pciAddress)
	err := os.WriteFile(path, []byte(ns), 0600)
	if err != nil {
		return fmt.Errorf("failed to write used PCI address lock file in the path(%q): %v", path, err)
	}

	return err
}

// DeleteAllocatedPCI Remove the allocated PCI file
// return error if the file doesn't exist
func (p *PCIAllocator) DeleteAllocatedPCI(pciAddress string) error {
	path := filepath.Join(p.dataDir, pciAddress)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("error removing PCI address lock file %s: %v", path, err)
	}
	return nil
}

// IsAllocated checks if the PCI address file exist
// if it exists we also check the network namespace still exist if not we delete the allocation
// The function will return an error if the pci is still allocated to a running pod
func (p *PCIAllocator) IsAllocated(pciAddress string) (bool, error) {
	path := filepath.Join(p.dataDir, pciAddress)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to check for pci address file for %s: %v", path, err)
	}

	dat, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("failed to read for pci address file for %s: %v", path, err)
	}

	// To prevent a locking of a PCI address for every pciAddress file we also add the netns path where it's been used
	// This way if for some reason the cmdDel command was not called but the pod namespace doesn't exist anymore
	// we release the PCI address
	networkNamespace, err := ns.GetNS(string(dat))
	if err != nil {
		logging.Debug("Mark the PCI address as released",
			"func", "IsAllocated",
			"pciAddress", pciAddress)
		err = p.DeleteAllocatedPCI(pciAddress)
		if err != nil {
			return false, fmt.Errorf("error deleting the pci allocation for vf pci address %s: %v", pciAddress, err)
		}

		return false, nil
	}

	// Close the network namespace
	networkNamespace.Close()
	return true, nil
}
