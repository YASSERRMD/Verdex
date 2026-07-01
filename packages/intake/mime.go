package intake

import (
	"fmt"
	"net/http"
)

// AllowedMIMETypes is the exhaustive list of MIME types accepted by the intake
// service.  Any upload whose detected or declared type is not in this list will
// be rejected before any bytes reach the TempBuffer.
var AllowedMIMETypes = []string{
	// Documents
	"application/pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"text/plain",

	// Audio
	"audio/mpeg",
	"audio/wav",
	"audio/ogg",

	// Video
	"video/mp4",

	// Images
	"image/png",
	"image/jpeg",
	"image/tiff",
	"image/webp",
}

// allowedMIMESet is a fast-lookup copy of AllowedMIMETypes.
var allowedMIMESet map[string]struct{}

func init() {
	allowedMIMESet = make(map[string]struct{}, len(AllowedMIMETypes))
	for _, m := range AllowedMIMETypes {
		allowedMIMESet[m] = struct{}{}
	}
}

// ValidateMIME returns an error if mimeType is not in AllowedMIMETypes.
func ValidateMIME(mimeType string) error {
	if _, ok := allowedMIMESet[mimeType]; !ok {
		return fmt.Errorf("intake: MIME type %q is not permitted; allowed types: %v", mimeType, AllowedMIMETypes)
	}
	return nil
}

// ValidateSizeMB returns an error if the payload exceeds limitMB megabytes.
// A limitMB of 0 disables the check.
func ValidateSizeMB(sizeBytes int64, limitMB int) error {
	if limitMB <= 0 {
		return nil
	}
	limitBytes := int64(limitMB) * 1024 * 1024
	if sizeBytes > limitBytes {
		return fmt.Errorf("intake: payload size %d bytes exceeds limit of %d MB (%d bytes)",
			sizeBytes, limitMB, limitBytes)
	}
	return nil
}

// DetectMIME sniffs up to the first 512 bytes of header to determine the
// actual MIME type of the payload.  It uses the standard library's content
// detection algorithm (identical to what browsers use).  If detection fails or
// produces an empty string, "application/octet-stream" is returned.
func DetectMIME(header []byte) string {
	if len(header) == 0 {
		return "application/octet-stream"
	}
	// Use at most 512 bytes for sniffing.
	sniff := header
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	mime := http.DetectContentType(sniff)
	if mime == "" {
		return "application/octet-stream"
	}
	// http.DetectContentType may append "; charset=utf-8" etc.  Strip params
	// so callers always get a bare type/subtype string for map lookups.
	for i := 0; i < len(mime); i++ {
		if mime[i] == ';' {
			mime = mime[:i]
			break
		}
	}
	// Trim trailing whitespace that may appear after stripping the parameter.
	for len(mime) > 0 && (mime[len(mime)-1] == ' ' || mime[len(mime)-1] == '\t') {
		mime = mime[:len(mime)-1]
	}
	return mime
}
