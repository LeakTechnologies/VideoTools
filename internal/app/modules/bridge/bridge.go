package bridge

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../internal/ffi/target/release -lvt_ffi -lntdll
#include <stdlib.h>
char* process_media_metadata(const char* input_path);
void free_rust_string(char* ptr);
*/
import "C"
import (
	"errors"
	"unsafe"
)

// InterceptMediaPayload handles the data extraction across the boundary seam.
func InterceptMediaPayload(path string) (string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// Call out to the Rust compiled static archive.
	cResult := C.process_media_metadata(cPath)
	if cResult == nil {
		return "", errors.New("boundary execution failed: rust interface returned a null pointer")
	}
	defer C.free_rust_string(cResult)

	// Convert back into standard Go memory allocations.
	return C.GoString(cResult), nil
}
