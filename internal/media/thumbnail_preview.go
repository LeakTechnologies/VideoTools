//go:build native_media

package media

import "image"

func (v *VideoPlayer) AddThumbnailFrame(time float64, frame *image.RGBA) {
	if frame == nil {
		return
	}
	v.thumbnailMu.Lock()
	defer v.thumbnailMu.Unlock()

	pts := int64(time * 1000)
	if v.thumbnailCache == nil {
		v.thumbnailCache = make(map[int64]*image.RGBA)
	}

	if len(v.thumbnailCache) >= 50 {
		var oldest int64
		for k := range v.thumbnailCache {
			if oldest == 0 || k < oldest {
				oldest = k
			}
		}
		delete(v.thumbnailCache, oldest)
	}

	v.thumbnailCache[pts] = frame
}

func (v *VideoPlayer) ClearThumbnailCache() {
	v.thumbnailMu.Lock()
	defer v.thumbnailMu.Unlock()
	v.thumbnailCache = make(map[int64]*image.RGBA)
}