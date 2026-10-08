use std::ffi::{CStr, CString};
use std::os::raw::c_char;

/// # Safety
///
/// `input_path` must be non-null and point to a valid, NUL-terminated C
/// string that outlives the call. The returned pointer is owned by the
/// caller and must be released with `free_rust_string`.
#[no_mangle]
pub unsafe extern "C" fn process_media_metadata(input_path: *const c_char) -> *mut c_char {
    if input_path.is_null() {
        return std::ptr::null_mut();
    }

    // SAFETY: Input pointer is verified non-null. The caller must guarantee it points to a valid, null-terminated C string.
    let c_str = unsafe { CStr::from_ptr(input_path) };
    let path_str = match c_str.to_str() {
        Ok(s) => s,
        Err(_) => return std::ptr::null_mut(),
    };

    let output_summary = format!("Rust Processing Validated for Source Profile: {}", path_str);

    match CString::new(output_summary) {
        Ok(c_string) => c_string.into_raw(),
        Err(_) => std::ptr::null_mut(),
    }
}

/// # Safety
///
/// `ptr` must be null or a pointer previously returned by this library via
/// `CString::into_raw`. Any other origin is undefined behaviour.
#[no_mangle]
pub unsafe extern "C" fn free_rust_string(ptr: *mut c_char) {
    if !ptr.is_null() {
        // SAFETY: The pointer must originate from CString::into_raw inside this library module context.
        unsafe { drop(CString::from_raw(ptr)) };
    }
}
