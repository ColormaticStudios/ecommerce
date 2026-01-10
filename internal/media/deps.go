package media

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/h2non/bimg"
)

func CheckDependencies() error {
	missing := make([]string, 0, 3)

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		missing = append(missing, "ffmpeg")
	}

	if !bimg.VipsIsTypeSupported(bimg.JPEG) {
		missing = append(missing, "libvips")
	} else if !bimg.VipsIsTypeSupportedSave(bimg.WEBP) {
		missing = append(missing, "libvips-webp")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing or unsupported dependencies: %s", strings.Join(missing, ", "))
	}

	return nil
}
