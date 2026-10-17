package main

import (
	"bytes"
	"fmt"
	"github.com/egoist/mygo/ui"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

func decodePreview(data []byte) (*ui.Bitmap, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 24_000_000 {
		return nil, fmt.Errorf("image dimensions exceed the preview limit")
	}
	return ui.DecodeBitmap(data)
}
