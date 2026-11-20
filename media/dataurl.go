package media

import (
	"encoding/base64"
	"errors"
	"strings"
)

// ImageFromDataURL decodes a "data:image/<format>;base64,..." URL.
func ImageFromDataURL(dataURL string) (Image, error) {
	header, payload, ok := strings.Cut(dataURL, ",")
	if !ok || !strings.HasPrefix(header, "data:image/") || !strings.HasSuffix(header, ";base64") {
		return Image{}, errors.New("not a base64 image data URL")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return Image{}, err
	}
	format := strings.TrimSuffix(strings.TrimPrefix(header, "data:image/"), ";base64")
	return Image{Data: data, Format: format}, nil
}
