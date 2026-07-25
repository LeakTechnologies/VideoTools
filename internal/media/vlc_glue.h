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
 * Go export functions (defined in vlc_events.go).
 */
extern void goVLCEventEOF(void* opaque);
extern void goVLCEventPositionChanged(void* opaque, float position);
extern void goVLCEventError(void* opaque);

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

/*
 * vlcAttachEvents — subscribe to player events and forward to Go callbacks.
 * Called once after the media player is created.
 */
static void vlcAttachEvents(libvlc_media_player_t* mp, void* userdata) {
    libvlc_event_manager_t* em = libvlc_media_player_event_manager(mp);

    libvlc_event_attach(em, libvlc_MediaPlayerEndReached,
        (libvlc_callback_t)goVLCEventEOF, userdata);
    libvlc_event_attach(em, libvlc_MediaPlayerPositionChanged,
        (libvlc_callback_t)goVLCEventPositionChanged, userdata);
    libvlc_event_attach(em, libvlc_MediaPlayerEncounteredError,
        (libvlc_callback_t)goVLCEventError, userdata);
}

/*
 * vlcDetachEvents — unsubscribe from player events.
 * Called once before releasing the media player.
 */
static void vlcDetachEvents(libvlc_media_player_t* mp, void* userdata) {
    libvlc_event_manager_t* em = libvlc_media_player_event_manager(mp);

    libvlc_event_detach(em, libvlc_MediaPlayerEndReached,
        (libvlc_callback_t)goVLCEventEOF, userdata);
    libvlc_event_detach(em, libvlc_MediaPlayerPositionChanged,
        (libvlc_callback_t)goVLCEventPositionChanged, userdata);
    libvlc_event_detach(em, libvlc_MediaPlayerEncounteredError,
        (libvlc_callback_t)goVLCEventError, userdata);
}

#endif /* VLC_GLUE_H */
