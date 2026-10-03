package server

import "mime"

// Only passive media is served on the authenticated AuraGo origin.
func passiveProxyContentType(value string) bool {
	kind, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	switch kind {
	case "application/vnd.apple.mpegurl", "application/x-mpegurl", "application/dash+xml", "application/octet-stream", "video/mp2t", "video/mp4", "video/webm", "video/ogg", "audio/mpeg", "audio/mp4", "audio/ogg", "audio/wav", "audio/aac", "audio/flac", "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif", "multipart/x-mixed-replace":
		return true
	}
	return false
}
