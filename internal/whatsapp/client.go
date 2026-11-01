package whatsapp

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// linkedDeviceName is what the phone lists under Linked devices.
const linkedDeviceName = "AuraGo"

// pairClientDisplay is the companion name WhatsApp validates for pairing
// codes. It must have the form "Browser (OS)".
const pairClientDisplay = "Chrome (Linux)"

// waClient is the narrow transport the channel uses. whatsmeowClient wraps the
// library; tests use fakeClient. Stage 2 adds receipts, chat presence, LID
// lookups and the login time; stage 3 adds media.
type waClient interface {
	AddEventHandler(handler func(evt any)) uint32
	ConnectContext(ctx context.Context) error
	Disconnect()
	IsConnected() bool
	IsLoggedIn() bool
	GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error)
	PairPhone(ctx context.Context, phone string) (string, error)
	Logout(ctx context.Context) error
	SendMessage(ctx context.Context, to types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error)
	SendPresence(ctx context.Context, state types.Presence) error
	OwnJID() types.JID
	MarkRead(ctx context.Context, ids []types.MessageID, timestamp time.Time, chat, sender types.JID) error
	SendChatPresence(ctx context.Context, chat types.JID, state types.ChatPresence, media types.ChatPresenceMedia) error
	// OwnLID is the linked account's LID without a device part, or EmptyJID before the first login.
	OwnLID() types.JID
	// PNForLID maps a LID to its phone JID through Store.LIDs. An unknown LID returns EmptyJID and no error.
	PNForLID(ctx context.Context, lid types.JID) (types.JID, error)
	// LastSuccessfulConnect is the time whatsmeow accepted the current login. Read it only from an
	// event handler: whatsmeow writes it on its handler queue without a lock (connectionevents.go:164).
	LastSuccessfulConnect() time.Time
}

// deviceStore is one opened linked-device database.
type deviceStore interface {
	Linked() bool
	OwnJID() types.JID
	NewClient() waClient
	DeleteLocal(ctx context.Context) error
	Close() error
}

// backend opens device stores. sqliteBackend is the production backend.
type backend interface {
	open(ctx context.Context, dataDir string) (deviceStore, error)
}

type sqliteBackend struct{ log waLog.Logger }

func (b sqliteBackend) open(ctx context.Context, dataDir string) (deviceStore, error) {
	s, err := openDeviceStore(ctx, dataDir, b.log)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// NewClient binds a whatsmeow client to the store's device.
func (s *sqliteDeviceStore) NewClient() waClient { return newWhatsmeowClient(s.device, s.log) }

var devicePropsOnce sync.Once

// configureDeviceProps names the linked device "AuraGo". It changes
// whatsmeow's process-wide defaults, so it runs once, before the first client.
// RequireFullSync stays false: no full history sync is requested.
func configureDeviceProps() {
	devicePropsOnce.Do(func() {
		store.SetOSInfo(linkedDeviceName, [3]uint32{1, 0, 0})
		store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_CHROME.Enum()
	})
}

type whatsmeowClient struct{ cli *whatsmeow.Client }

// maxAutoReconnectErrors caps whatsmeow's own reconnect backoff. The library
// waits AutoReconnectErrors*2s between attempts without an upper bound, so 150
// keeps the delay at or below 300 s, the same five-minute cap the Bot uses.
const maxAutoReconnectErrors = 150

// capAutoReconnectErrors clamps whatsmeow's reconnect counter and tells it to
// keep retrying.
func capAutoReconnectErrors(errs *int) bool {
	if *errs > maxAutoReconnectErrors {
		*errs = maxAutoReconnectErrors
	}
	return true
}

func newWhatsmeowClient(device *store.Device, log waLog.Logger) *whatsmeowClient {
	configureDeviceProps()
	cli := whatsmeow.NewClient(device, log)
	cli.EnableAutoReconnect = true
	cli.AutoReconnectHook = func(error) bool { return capAutoReconnectErrors(&cli.AutoReconnectErrors) }
	return &whatsmeowClient{cli: cli}
}

func (c *whatsmeowClient) AddEventHandler(handler func(evt any)) uint32 {
	return c.cli.AddEventHandler(handler)
}

func (c *whatsmeowClient) ConnectContext(ctx context.Context) error { return c.cli.ConnectContext(ctx) }
func (c *whatsmeowClient) Disconnect()                              { c.cli.Disconnect() }
func (c *whatsmeowClient) IsConnected() bool                        { return c.cli.IsConnected() }
func (c *whatsmeowClient) IsLoggedIn() bool                         { return c.cli.IsLoggedIn() }

func (c *whatsmeowClient) GetQRChannel(ctx context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	return c.cli.GetQRChannel(ctx)
}

func (c *whatsmeowClient) PairPhone(ctx context.Context, phone string) (string, error) {
	return c.cli.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, pairClientDisplay)
}

func (c *whatsmeowClient) Logout(ctx context.Context) error { return c.cli.Logout(ctx) }

func (c *whatsmeowClient) SendMessage(ctx context.Context, to types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
	return c.cli.SendMessage(ctx, to, msg)
}

func (c *whatsmeowClient) SendPresence(ctx context.Context, state types.Presence) error {
	return c.cli.SendPresence(ctx, state)
}

func (c *whatsmeowClient) OwnJID() types.JID { return c.cli.Store.GetJID().ToNonAD() }

func (c *whatsmeowClient) MarkRead(ctx context.Context, ids []types.MessageID, timestamp time.Time, chat, sender types.JID) error {
	return c.cli.MarkRead(ctx, ids, timestamp, chat, sender)
}

func (c *whatsmeowClient) SendChatPresence(ctx context.Context, chat types.JID, state types.ChatPresence, media types.ChatPresenceMedia) error {
	return c.cli.SendChatPresence(ctx, chat, state, media)
}

func (c *whatsmeowClient) OwnLID() types.JID { return c.cli.Store.GetLID().ToNonAD() }

func (c *whatsmeowClient) PNForLID(ctx context.Context, lid types.JID) (types.JID, error) {
	pn, err := c.cli.Store.LIDs.GetPNForLID(ctx, lid.ToNonAD())
	return pn.ToNonAD(), err
}

func (c *whatsmeowClient) LastSuccessfulConnect() time.Time { return c.cli.LastSuccessfulConnect }
