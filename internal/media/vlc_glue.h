#ifndef VLC_GLUE_H
#define VLC_GLUE_H

#include <vlc/vlc.h>

/*
 * Go export functions (defined in vlc_video.go).
 */
extern void* goVLCLockCB(void* opaque, void** planes);
extern void  goVLCUnlockCB(void* opaque, void* picture, void* const* planes);
extern void  goVLCDisplayCB(void* opaque, void* picture);
extern unsigned goVLCFormatSetup(void** opaque, char* chroma,
                                  unsigned* width, unsigned* height,
                                  unsigned* pitches, unsigned* lines);

/*
 * vlcSetCallbacks — bridge for libvlc_video_set_callbacks.
 */
static void vlcSetCallbacks(libvlc_media_player_t* mp, void* userdata) {
    libvlc_video_set_callbacks(mp,
        goVLCLockCB, goVLCUnlockCB, goVLCDisplayCB, userdata);
}

/*
 * vlcSetFormatCallbacks — bridge for libvlc_video_set_format_callbacks.
 */
static void vlcSetFormatCallbacks(libvlc_media_player_t* mp, void* userdata) {
    libvlc_video_set_format_callbacks(mp, goVLCFormatSetup, NULL);
}

#endif /* VLC_GLUE_H */
