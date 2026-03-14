package thumbnail

// LogoData holds the app logo PNG bytes, set at startup via SetLogoData.
var LogoData []byte

// FontData holds the monospace font TTF bytes, set at startup via SetFontData.
var FontData []byte

// SetLogoData sets the logo image bytes used for contact sheet headers.
func SetLogoData(data []byte) { LogoData = data }

// SetFontData sets the font bytes used for text overlays in thumbnails.
func SetFontData(data []byte) { FontData = data }
