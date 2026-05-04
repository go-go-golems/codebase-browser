//go:build embed

package staticapp

import (
	"embed"
	"io/fs"
)

// embeddedAssets contains the built Vite static export assets. The build
// pipeline copies ui/dist/public into internal/staticapp/embed/public before
// compiling with -tags embed, making review export portable outside the source
// checkout.
//
//go:embed embed/public
var embeddedAssets embed.FS

func spaAssetsFS() (fs.FS, bool, error) {
	sub, err := fs.Sub(embeddedAssets, "embed/public")
	return sub, true, err
}
