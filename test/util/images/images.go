package images

import (
	"fmt"
)

var (
	registry  string
	testImage string
)

func init() {
	registry = "docker.io"
	testImage = "alpine"
}

// GetPodTestImage returns image to be used during testing
func GetPodTestImage() string {
	return fmt.Sprintf("%s/%s", registry, testImage)
}

// SetImageRegistry - set image registry
func SetImageRegistry(imgRegistry string) {
	registry = imgRegistry
}

// SetTestImageName - set test image name
func SetTestImageName(name string) {
	testImage = name
}

// ResetToDefaults - reset values to default state
func ResetToDefaults() {
	registry = "docker.io"
	testImage = "alpine"
}
