package thumbnail

// LogoData holds the app logo PNG bytes, set at startup via SetLogoData.
var LogoData []byte

// FontData holds the regular monospace font TTF bytes.
var FontData []byte

// BoldFontData holds the bold monospace font TTF bytes.
var BoldFontData []byte

// SetLogoData sets the logo image bytes used for contact sheet headers.
func SetLogoData(data []byte) { LogoData = data }

// SetFontData sets the regular font bytes used for text overlays in thumbnails.
func SetFontData(data []byte) { FontData = data }

// SetBoldFontData sets the bold font bytes used for the title line in contact sheet headers.
func SetBoldFontData(data []byte) { BoldFontData = data }
