package server

import (
	"context"
	"io"
	"reflect"
	"time"

	"aurago/internal/config"
	"aurago/internal/mailer"
)

type mailerGuardedAdapter struct {
	inner    mailer.Adapter
	server   *Server
	id       string
	identity any
}

func mailerSourceIdentity(cfg *config.Config, account mailer.Account) any {
	if cfg == nil {
		return nil
	}
	switch account.Source {
	case "email":
		for _, entry := range cfg.EmailAccounts {
			if "email:"+entry.ID == account.ID {
				if entry.OAuthSource == "gog" {
					parent, _ := cfg.Gog.Account(entry.OAuthAccountID)
					return struct {
						Account                    config.EmailAccount
						Parent                     config.GogAccount
						Enabled, ReadOnly, Network bool
					}{entry, parent, cfg.Gog.Enabled, cfg.Gog.ReadOnly, cfg.Agent.AllowNetworkRequests}
				}
				return entry
			}
		}
	case "gog":
		for _, entry := range cfg.Gog.Accounts {
			if "gog:"+entry.ID == account.ID {
				return struct {
					Account                    config.GogAccount
					Enabled, ReadOnly, Network bool
				}{entry, cfg.Gog.Enabled, cfg.Gog.ReadOnly, cfg.Agent.AllowNetworkRequests}
			}
		}
	case "google_workspace":
		return struct {
			Settings any
			Network  bool
		}{cfg.GoogleWorkspace, cfg.Agent.AllowNetworkRequests}
	case "agentmail":
		return cfg.AgentMail
	}
	return nil
}

func (a *mailerGuardedAdapter) begin(parent context.Context, capability string, write bool) (context.Context, context.CancelFunc, error) {
	allowed := func() error {
		cfg := a.server.ConfigSnapshot()
		if cfg == nil || !cfg.VirtualDesktop.Enabled {
			return mailer.ErrDisabled
		}
		account, err := a.server.mailerSource(parent, a.id)
		if err != nil {
			return err
		}
		if account.Disabled || !reflect.DeepEqual(a.identity, mailerSourceIdentity(cfg, account)) {
			return mailer.ErrDisabled
		}
		if write && (account.ReadOnly || cfg.VirtualDesktop.ReadOnly) {
			return mailer.ErrReadOnly
		}
		for _, cap := range account.Capabilities {
			if cap == capability {
				return nil
			}
		}
		return mailer.ErrCapability
	}
	if err := allowed(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	// Account revocation must also interrupt an already blocked protocol call.
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if allowed() != nil {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel, nil
}

func (a *mailerGuardedAdapter) Folders(ctx context.Context) ([]mailer.Folder, error) {
	ctx, cancel, err := a.begin(ctx, "folders_read", false)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return a.inner.Folders(ctx)
}
func (a *mailerGuardedAdapter) Sync(ctx context.Context, folder, cursor string) (mailer.SyncResult, error) {
	ctx, cancel, err := a.begin(ctx, "read", false)
	if err != nil {
		return mailer.SyncResult{}, err
	}
	defer cancel()
	return a.inner.Sync(ctx, folder, cursor)
}

func (a *mailerGuardedAdapter) StartSyncSession(ctx context.Context) (func(), error) {
	session, ok := a.inner.(mailer.SyncSession)
	if !ok {
		return func() {}, nil
	}
	ctx, cancel, err := a.begin(ctx, "read", false)
	if err != nil {
		return nil, err
	}
	stop, err := session.StartSyncSession(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	return func() {
		if stop != nil {
			stop()
		}
		cancel()
	}, nil
}
func (a *mailerGuardedAdapter) Read(ctx context.Context, folder, id string) (mailer.Message, error) {
	ctx, cancel, err := a.begin(ctx, "read", false)
	if err != nil {
		return mailer.Message{}, err
	}
	defer cancel()
	return a.inner.Read(ctx, folder, id)
}
func (a *mailerGuardedAdapter) Raw(ctx context.Context, folder, id string) ([]byte, error) {
	ctx, cancel, err := a.begin(ctx, "read", false)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return a.inner.Raw(ctx, folder, id)
}
func (a *mailerGuardedAdapter) Attachment(ctx context.Context, folder, id, attachment string) (io.ReadCloser, error) {
	ctx, cancel, err := a.begin(ctx, "read", false)
	if err != nil {
		return nil, err
	}
	r, err := a.inner.Attachment(ctx, folder, id, attachment)
	if err != nil {
		cancel()
		return nil, err
	}
	return &mailerAttachmentReader{ReadCloser: r, cancel: cancel}, nil
}

type mailerAttachmentReader struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *mailerAttachmentReader) Close() error { defer r.cancel(); return r.ReadCloser.Close() }
func (a *mailerGuardedAdapter) Mutate(ctx context.Context, folder, id string, m mailer.Mutation) error {
	cap := string(m.Kind)
	switch m.Kind {
	case mailer.MutationRead, mailer.MutationUnread:
		cap = "read_state"
	case mailer.MutationUnstar:
		cap = "star"
	case "label_add", "label_remove":
		cap = "labels_write"
	}
	ctx, cancel, err := a.begin(ctx, cap, true)
	if err != nil {
		return err
	}
	defer cancel()
	return a.inner.Mutate(ctx, folder, id, m)
}
func (a *mailerGuardedAdapter) Send(ctx context.Context, out mailer.Outgoing) (mailer.SendResult, error) {
	ctx, cancel, err := a.begin(ctx, "send", true)
	if err != nil {
		return mailer.SendResult{Rejected: true}, err
	}
	defer cancel()
	return a.inner.Send(ctx, out)
}
func (a *mailerGuardedAdapter) Test(ctx context.Context) error {
	ctx, cancel, err := a.begin(ctx, "read", false)
	if err != nil {
		return err
	}
	defer cancel()
	return a.inner.Test(ctx)
}
func (a *mailerGuardedAdapter) TestSend(ctx context.Context) error {
	ctx, cancel, err := a.begin(ctx, "send", false)
	if err != nil {
		return err
	}
	defer cancel()
	probe, ok := a.inner.(interface{ TestSend(context.Context) error })
	if !ok {
		return mailer.ErrCapability
	}
	return probe.TestSend(ctx)
}
func (a *mailerGuardedAdapter) Profile(ctx context.Context) (string, error) {
	ctx, cancel, err := a.begin(ctx, "read", false)
	if err != nil {
		return "", err
	}
	defer cancel()
	p, ok := a.inner.(interface {
		Profile(context.Context) (string, error)
	})
	if !ok {
		return "", nil
	}
	return p.Profile(ctx)
}
func (a *mailerGuardedAdapter) AppendSent(ctx context.Context, out mailer.Outgoing, result mailer.SendResult) error {
	ctx, cancel, err := a.begin(ctx, "send", true)
	if err != nil {
		return err
	}
	defer cancel()
	p, ok := a.inner.(mailer.SentAppender)
	if !ok {
		return nil
	}
	return p.AppendSent(ctx, out, result)
}

func (a *mailerGuardedAdapter) AppendSentTo(ctx context.Context, out mailer.Outgoing, result mailer.SendResult, folder mailer.Folder) error {
	ctx, cancel, err := a.begin(ctx, "send", true)
	if err != nil {
		return err
	}
	defer cancel()
	if folder.AccountID != "" && folder.AccountID != a.id {
		return mailer.ErrInvalid
	}
	if p, ok := a.inner.(mailer.MappedSentAppender); ok {
		return p.AppendSentTo(ctx, out, result, folder)
	}
	if p, ok := a.inner.(mailer.SentAppender); ok {
		if folder.ID != "" {
			return mailer.ErrCapability
		}
		return p.AppendSent(ctx, out, result)
	}
	return nil
}
func (a *mailerGuardedAdapter) Search(ctx context.Context, folder string, q mailer.MessageQuery) (mailer.MessagePage, error) {
	ctx, cancel, err := a.begin(ctx, "search", false)
	if err != nil {
		return mailer.MessagePage{}, err
	}
	defer cancel()
	p, ok := a.inner.(mailer.Searcher)
	if !ok {
		return mailer.MessagePage{}, mailer.ErrCapability
	}
	return p.Search(ctx, folder, q)
}
func (a *mailerGuardedAdapter) CreateFolder(ctx context.Context, name, parent string) (mailer.Folder, error) {
	ctx, cancel, err := a.begin(ctx, "folders_create", true)
	if err != nil {
		return mailer.Folder{}, err
	}
	defer cancel()
	p, ok := a.inner.(interface {
		CreateFolder(context.Context, string, string) (mailer.Folder, error)
	})
	if !ok {
		return mailer.Folder{}, mailer.ErrCapability
	}
	return p.CreateFolder(ctx, name, parent)
}
