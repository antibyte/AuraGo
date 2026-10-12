package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"aurago/internal/cydcreator"
)

// cydImportBodySlop is the multipart overhead allowed around an import ZIP.
const cydImportBodySlop = 1 << 20

// cydDeviceTokenPattern is the shape of a device token MintDevice issues
// (ccdt_ + 64 lowercase hex characters, 256 bits).
var cydDeviceTokenPattern = regexp.MustCompile(`^ccdt_[0-9a-f]{64}$`)

// cydDeviceTokenPresent reports a request that carries exactly one bearer
// credential shaped like a device token. Session cookies, API tokens of any
// scope and malformed credentials never are; the service still checks the
// token hash, the device binding and the failed-authentication limit.
func cydDeviceTokenPresent(r *http.Request) bool {
	token, _ := bearerCredential(r.Header.Get("Authorization"))
	return cydDeviceTokenPattern.MatchString(token)
}

// cydCreatorDeviceDesktopRoutes are the plan 5 desktop routes: device list,
// minting for the flasher, rename/OTA target, revoke, ZIP export and import.
func cydCreatorDeviceDesktopRoutes(s *Server) []cydRoute {
	return []cydRoute{
		{Method: http.MethodGet, Pattern: "projects/{id}/devices", Handler: s.cydListDevices},
		{Method: http.MethodPost, Pattern: "projects/{id}/devices", Write: true, Handler: s.cydMintDevice},
		{Method: http.MethodPatch, Pattern: "devices/{id}", Write: true, Handler: s.cydUpdateDevice},
		{Method: http.MethodDelete, Pattern: "devices/{id}", Write: true, Handler: s.cydRevokeDevice},
		{Method: http.MethodGet, Pattern: "projects/{id}/export", Handler: s.cydExportProject},
		{Method: http.MethodPost, Pattern: "import", Write: true, Handler: s.cydImportProject},
	}
}

// cydCreatorDeviceEndpointRoutes are the device endpoints under
// /api/cyd-creator/device/ (session-auth bypass; handleCYDCreatorDevice
// refuses requests without a device token before dispatch and every handler
// authenticates the token through the service).
func cydCreatorDeviceEndpointRoutes(s *Server) []cydRoute {
	return []cydRoute{
		{Method: http.MethodPost, Pattern: "heartbeat", Handler: s.cydDeviceHeartbeat},
		{Method: http.MethodGet, Pattern: "firmware/{id}", Handler: s.cydDeviceFirmware},
		{Method: http.MethodPost, Pattern: "ota-result", Handler: s.cydDeviceOTAResult},
	}
}

func (s *Server) cydListDevices(w http.ResponseWriter, r *http.Request, params map[string]string) {
	devices, err := s.CYDCreator.ListDevices(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (s *Server) cydMintDevice(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct {
		BuildID string `json:"build_id"`
		Name    string `json:"name"`
	}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	if !s.CYDCreator.Limits().OTA.Enabled {
		cydWriteError(w, http.StatusServiceUnavailable, cydcreator.CodeOTADisabled, "updates over Wi-Fi are switched off", nil)
		return
	}
	endpoint, err := cydCreatorDeviceEndpoint(r.Context(), s.ConfigSnapshot())
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	minted, err := s.CYDCreator.MintDevice(r.Context(), params["id"], cydcreator.MintDeviceInput{BuildID: body.BuildID, Name: body.Name, Endpoint: endpoint})
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusCreated, minted)
}

func (s *Server) cydUpdateDevice(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body cydcreator.DeviceUpdate
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	device, err := s.CYDCreator.UpdateDevice(r.Context(), params["id"], body)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"device": device})
}

func (s *Server) cydRevokeDevice(w http.ResponseWriter, r *http.Request, params map[string]string) {
	if err := s.CYDCreator.RevokeDevice(r.Context(), params["id"]); err != nil {
		s.cydError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cydExportProject(w http.ResponseWriter, r *http.Request, params map[string]string) {
	data, name, err := s.CYDCreator.ExportProject(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) cydImportProject(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	tooLarge := func() {
		cydWriteError(w, http.StatusUnprocessableEntity, cydcreator.CodeImportInvalid,
			fmt.Sprintf("the archive may have at most %d bytes", cydcreator.MaxImportBytes),
			map[string]any{"reason": "too_large", "limit": cydcreator.MaxImportBytes})
	}
	badRequest := func() {
		cydWriteError(w, http.StatusBadRequest, cydcreator.CodeBadRequest, `send one ZIP archive as multipart field "file"`, nil)
	}
	r.Body = http.MaxBytesReader(w, r.Body, cydcreator.MaxImportBytes+cydImportBodySlop)
	reader, err := r.MultipartReader()
	if err != nil {
		badRequest()
		return
	}
	for {
		part, err := reader.NextPart()
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			tooLarge()
			return
		case err != nil:
			badRequest()
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, cydcreator.MaxImportBytes+1))
		_ = part.Close()
		switch {
		case errors.As(err, &maxErr), err == nil && len(data) > cydcreator.MaxImportBytes:
			tooLarge()
			return
		case err != nil:
			badRequest()
			return
		}
		project, err := s.CYDCreator.ImportProject(r.Context(), data, part.FileName())
		if err != nil {
			s.cydError(w, r, err)
			return
		}
		cydWriteJSON(w, http.StatusCreated, map[string]any{"project": project})
		return
	}
}

// cydDeviceRequest collects the bearer token and the client address of a
// device request. The address follows the trusted-proxy rule (ClientIP), so
// devices behind a reverse proxy do not share one failed-authentication bucket.
func (s *Server) cydDeviceRequest(r *http.Request) cydcreator.DeviceRequest {
	token, _ := bearerCredential(r.Header.Get("Authorization"))
	cfg := s.ConfigSnapshot()
	return cydcreator.DeviceRequest{Token: token, ClientIP: ClientIP(r, cfg != nil && cfg.Server.HTTPS.BehindProxy)}
}

func (s *Server) cydDeviceHeartbeat(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var hb cydcreator.Heartbeat
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &hb) {
		return
	}
	resp, err := s.CYDCreator.DeviceHeartbeat(r.Context(), s.cydDeviceRequest(r), hb)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, resp)
}

func (s *Server) cydDeviceFirmware(w http.ResponseWriter, r *http.Request, params map[string]string) {
	file, info, err := s.CYDCreator.DeviceFirmware(r.Context(), s.cydDeviceRequest(r), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	// no-transform: a proxy must not compress or re-encode the image; the
	// firmware compares Content-Length with the offered size (plan 1b).
	w.Header().Set("Cache-Control", "no-store, no-transform")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-CYD-SHA256", info.SHA256)
	// ServeContent sets Content-Length (the reader is cut to the recorded size).
	cydServeReader(w, r, info.Name, file)
}

func (s *Server) cydDeviceOTAResult(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var res cydcreator.OTAResult
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &res) {
		return
	}
	if err := s.CYDCreator.DeviceOTAResult(r.Context(), s.cydDeviceRequest(r), res); err != nil {
		s.cydError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// cydCreatorOTACapability replaces the OTA block of Capabilities with the
// resolved device URL (configured or derived), so the OTA dialog can show
// where devices connect and warn about plain http://.
func cydCreatorOTACapability(caps *cydcreator.Capabilities, base string) {
	if base == "" {
		return
	}
	caps.OTA.PublicURL = base
	caps.OTA.Insecure = strings.HasPrefix(strings.ToLower(base), "http://")
}
