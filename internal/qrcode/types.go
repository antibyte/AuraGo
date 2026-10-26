// Package qrcode provides bounded, local QR encoding, rendering and decoding.
// It accepts resource bytes and never reads files, fetches URLs or publishes artifacts.
package qrcode

import "context"

const (
	SchemaVersion      = 1
	MaxResourceBytes   = 10 << 20
	MaxInputPixels     = 32_000_000
	MaxOutputDimension = 4096
	MaxCSVBytes        = 5 << 20
	MaxBatchRows       = 500
	MaxBatchBytes      = 128 << 20
	MaxPayloadBytes    = 16 << 10
)

type QRSpec struct {
	SchemaVersion int     `json:"schema_version"`
	Content       Content `json:"content"`
	Style         Style   `json:"style"`
	Logo          *Logo   `json:"logo,omitempty"`
	Frame         *Frame  `json:"frame,omitempty"`
	Output        Output  `json:"output"`
}

type Content struct {
	Type      string            `json:"type"`
	Text      string            `json:"text,omitempty"`
	URL       string            `json:"url,omitempty"`
	Email     *EmailContent     `json:"email,omitempty"`
	Telephone *TelephoneContent `json:"telephone,omitempty"`
	SMS       *SMSContent       `json:"sms,omitempty"`
	WiFi      *WiFiContent      `json:"wifi,omitempty"`
	VCard     *VCardContent     `json:"vcard,omitempty"`
	Location  *LocationContent  `json:"location,omitempty"`
	Calendar  *CalendarContent  `json:"calendar,omitempty"`
	EPC       *EPCContent       `json:"epc,omitempty"`
}

type EmailContent struct {
	To      []string `json:"to"`
	Subject string   `json:"subject,omitempty"`
	Body    string   `json:"body,omitempty"`
	CC      []string `json:"cc,omitempty"`
	BCC     []string `json:"bcc,omitempty"`
}

type TelephoneContent struct {
	Number string `json:"number"`
}
type SMSContent struct {
	Number string `json:"number"`
	Body   string `json:"body,omitempty"`
}
type WiFiContent struct {
	SSID     string `json:"ssid"`
	Password string `json:"password,omitempty"`
	Security string `json:"security"`
	Hidden   bool   `json:"hidden,omitempty"`
}
type VCardContent struct {
	Version       string          `json:"version"`
	FirstName     string          `json:"first_name,omitempty"`
	LastName      string          `json:"last_name,omitempty"`
	FormattedName string          `json:"formatted_name,omitempty"`
	MiddleName    string          `json:"middle_name,omitempty"`
	Prefix        string          `json:"prefix,omitempty"`
	Suffix        string          `json:"suffix,omitempty"`
	Organization  string          `json:"organization,omitempty"`
	Title         string          `json:"title,omitempty"`
	Phone         string          `json:"phone,omitempty"`
	PhoneWork     string          `json:"phone_work,omitempty"`
	PhonePrivate  string          `json:"phone_private,omitempty"`
	PhoneMobile   string          `json:"phone_mobile,omitempty"`
	Email         string          `json:"email,omitempty"`
	URL           string          `json:"url,omitempty"`
	Note          string          `json:"note,omitempty"`
	Birthday      string          `json:"birthday,omitempty"`
	Address       *AddressContent `json:"address,omitempty"`
}
type AddressContent struct {
	Street     string `json:"street,omitempty"`
	City       string `json:"city,omitempty"`
	Region     string `json:"region,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country,omitempty"`
}
type LocationContent struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type CalendarContent struct {
	CreatedAt   string `json:"created_at,omitempty"`
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Timezone    string `json:"timezone,omitempty"`
	AllDay      bool   `json:"all_day,omitempty"`
	Location    string `json:"location,omitempty"`
	Description string `json:"description,omitempty"`
}
type EPCContent struct {
	Version     string `json:"version"`
	Name        string `json:"name"`
	IBAN        string `json:"iban"`
	BIC         string `json:"bic,omitempty"`
	Amount      string `json:"amount,omitempty"`
	Purpose     string `json:"purpose,omitempty"`
	Reference   string `json:"reference,omitempty"`
	Text        string `json:"text,omitempty"`
	Information string `json:"information,omitempty"`
}

type Style struct {
	Size             int     `json:"size"`
	QuietZone        *int    `json:"quiet_zone,omitempty"`
	ErrorCorrection  string  `json:"error_correction"`
	ModuleShape      string  `json:"module_shape"`
	FinderShape      string  `json:"finder_shape"`
	CenterShape      string  `json:"center_shape"`
	ModuleColor      string  `json:"module_color"`
	FinderColor      string  `json:"finder_color"`
	CenterColor      string  `json:"center_color"`
	BackgroundColor  string  `json:"background_color"`
	BackgroundRadius float64 `json:"background_radius"`
}

type ImageResource struct {
	MIME string `json:"mime"`
	Data []byte `json:"data"`
}
type Logo struct {
	Image                 ImageResource `json:"image"`
	SizePercent           float64       `json:"size_percent"`
	Margin                int           `json:"margin"`
	HideBackgroundModules bool          `json:"hide_background_modules"`
}
type Frame struct {
	Position        string         `json:"position"`
	Text            string         `json:"text"`
	Font            string         `json:"font"`
	FontSize        int            `json:"font_size"`
	SideWidth       int            `json:"side_width"`
	TextColor       string         `json:"text_color"`
	BackgroundColor string         `json:"background_color"`
	BorderColor     string         `json:"border_color"`
	BorderWidth     int            `json:"border_width"`
	BorderRadius    float64        `json:"border_radius"`
	Padding         int            `json:"padding"`
	BackgroundImage *ImageResource `json:"background_image,omitempty"`
}
type Output struct {
	Format         string `json:"format"`
	Quality        int    `json:"quality"`
	JPEGBackground string `json:"jpeg_background"`
	TextMode       string `json:"text_mode,omitempty"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}
type Metadata struct {
	MIME        string       `json:"mime"`
	Extension   string       `json:"extension"`
	Width       int          `json:"width"`
	Height      int          `json:"height"`
	QRVersion   int          `json:"qr_version"`
	Modules     int          `json:"modules"`
	ModulePitch float64      `json:"module_pitch"`
	Verified    bool         `json:"verified"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}
type Result struct {
	Data     []byte   `json:"-"`
	Metadata Metadata `json:"metadata"`
}
type ScanResult struct {
	Text        string       `json:"text"`
	Type        string       `json:"type"`
	Content     *Content     `json:"content,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// FontProvider returns font bytes for a fixed catalog ID. It must not use an
// untrusted caption as a path or fetch a remote font while rendering.
type FontProvider func(context.Context, string) ([]byte, error)
type RenderOptions struct{ FontProvider FontProvider }

type Presentation struct {
	Style Style  `json:"style"`
	Logo  *Logo  `json:"logo,omitempty"`
	Frame *Frame `json:"frame,omitempty"`
}
type Preset struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Presentation Presentation `json:"presentation"`
}
type FontInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Resource string `json:"resource,omitempty"`
}
type Catalog struct {
	SchemaVersion  int        `json:"schema_version"`
	ContentTypes   []string   `json:"content_types"`
	ModuleShapes   []string   `json:"module_shapes"`
	FinderShapes   []string   `json:"finder_shapes"`
	CenterShapes   []string   `json:"center_shapes"`
	FramePositions []string   `json:"frame_positions"`
	OutputFormats  []string   `json:"output_formats"`
	Fonts          []FontInfo `json:"fonts"`
	Presets        []Preset   `json:"presets"`
}

type BatchOptions struct {
	ContentType string `json:"content_type"`
}
type RowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type BatchEntry struct {
	Preview  *ImageResource `json:"preview,omitempty"`
	Row      int            `json:"row"`
	Filename string         `json:"filename"`
	Metadata Metadata       `json:"metadata"`
}
type BatchResult struct {
	Data    []byte       `json:"-"`
	Entries []BatchEntry `json:"entries,omitempty"`
	Errors  []RowError   `json:"errors,omitempty"`
	Total   int          `json:"total"`
}
