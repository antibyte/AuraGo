package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"

	"aurago/internal/qrcode"
)

const linkQRSize = 384

// renderQRDataURI renders a WhatsApp link code as a PNG data URI that
// internal/qrcode has verified. The UI shows it like the TOTP enrollment QR.
// The payload is never logged.
func renderQRDataURI(ctx context.Context, payload string) (string, error) {
	result, err := qrcode.Render(ctx, qrcode.QRSpec{
		SchemaVersion: qrcode.SchemaVersion,
		Content:       qrcode.Content{Type: "text", Text: payload},
		Style:         qrcode.Style{Size: linkQRSize},
		Output:        qrcode.Output{Format: "png"},
	})
	if err != nil {
		return "", fmt.Errorf("render whatsapp link qr: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(result.Data), nil
}
