//go:build !embed

package staticapp

import (
	"io/fs"
	"os"
)

func spaAssetsFS() (fs.FS, bool, error) {
	return os.DirFS("ui/dist/public"), false, nil
}
