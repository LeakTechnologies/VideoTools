# GNOME/Linux Compatibility Notes

## Current Status
VideoTools is built with Fyne UI framework and runs on GNOME/Fedora and other Linux desktop environments.

## Known Issues

### Double-Click Titlebar to Maximize
**Issue**: Double-clicking the titlebar doesn't maximize the window like native GNOME apps.

**Cause**: This is a Fyne framework limitation. Fyne uses its own window rendering and doesn't fully implement all native window manager behaviors.

**Workarounds for Users**:
- Use GNOME's maximize button in titlebar
- Use keyboard shortcuts: `Super+Up` (GNOME default)
- Press `F11` for fullscreen (if app supports it)
- Right-click titlebar → Maximize

**Status**: Upstream Fyne issue. Monitor: https://github.com/fyne-io/fyne/issues

### Window Sizing
**Fixed**: Window now properly resizes and can be made smaller. Minimum sizes have been reduced to allow flexible layouts.

## Desktop Environment Testing

### Tested On
- ✅ GNOME (Fedora 43)
- ✅ X11 session
- ✅ Wayland session

### Should Work On (Untested)
- KDE Plasma
- XFCE
- Cinnamon
- MATE
- Other Linux DEs

## Cross-Platform Goals

VideoTools aims to run smoothly on:
- **Linux**: GNOME, KDE, XFCE, etc.
- **Windows**: Native Windows window behavior

## Fyne Framework Considerations

### Advantages
- Cross-platform by default
- Single codebase for all OSes
- Modern Go-based development
- Good performance

### Limitations
- Some native behaviors may differ
- Window management is abstracted
- Custom titlebar rendering
- Some OS-specific shortcuts may not work

## Future Improvements

### Short Term
- [x] Flexible window sizing
- [x] Better minimum size handling
- [ ] Document all keyboard shortcuts
- [ ] Test on more Linux DEs

### Long Term
- [ ] Consider native window decorations option
- [ ] Investigate Fyne improvements for window management
- [ ] Add more GNOME-like keyboard shortcuts
- [ ] Better integration with system theme

## Recommendations for Users

### GNOME Users
- Use Super key shortcuts for window management
- Maximize: `Super+Up`
- Snap left/right: `Super+Left/Right`
- Fullscreen: `F11` (if supported)
- Close: `Alt+F4` or `Ctrl+Q`

### General Linux Users
- Most window management shortcuts work via your window manager
- VideoTools respects window manager tiling
- Window can be resized freely
- Multiple instances can run simultaneously

## Development Notes

When adding features:
- Test on both X11 and Wayland
- Verify window resizing behavior
- Check keyboard shortcuts don't conflict
- Consider both mouse and keyboard workflows
- Test with HiDPI displays

## Reporting Issues

If you encounter GNOME/Linux specific issues:
1. Note your distro and desktop environment
2. Specify X11 or Wayland
3. Include window manager if using tiling WM
4. Provide steps to reproduce
5. Check if issue exists on other platforms

## Resources
- Fyne Documentation: https://developer.fyne.io/
- GNOME HIG: https://developer.gnome.org/hig/
- Linux Desktop Testing: Multiple VMs recommended
