package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"aurago/internal/config"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
)

// newClientOptionsContext installs an opener that allows the controller to
// cancel a socket while Paho is still waiting for CONNACK. Paho's Client
// interface does not expose that in-flight connection directly.
func newClientOptionsContext(ctx context.Context, cfg *config.Config, log *slog.Logger) (*pahomqtt.ClientOptions, error) {
	opts, err := newClientOptions(cfg, log)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	opts.SetCustomOpenConnectionFn(func(uri *url.URL, options pahomqtt.ClientOptions) (net.Conn, error) {
		return openCancelableMQTTConnection(ctx, uri, options)
	})
	return opts, nil
}

type cancelableConn struct {
	net.Conn
	closed chan struct{}
	once   sync.Once
}

func newCancelableConn(ctx context.Context, conn net.Conn) net.Conn {
	wrapped := &cancelableConn{Conn: conn, closed: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-wrapped.closed:
		}
	}()
	return wrapped
}

func (c *cancelableConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func openCancelableMQTTConnection(ctx context.Context, uri *url.URL, options pahomqtt.ClientOptions) (net.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if uri == nil {
		return nil, fmt.Errorf("MQTT broker URL is missing")
	}
	dialer := options.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	switch uri.Scheme {
	case "tcp", "mqtt":
		conn, err := dialer.DialContext(ctx, "tcp", uri.Host)
		if err != nil {
			return nil, err
		}
		return newCancelableConn(ctx, conn), nil
	case "ssl", "tls", "mqtts", "mqtt+ssl", "tcps":
		conn, err := dialer.DialContext(ctx, "tcp", uri.Host)
		if err != nil {
			return nil, err
		}
		tlsConfig := options.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		} else {
			copyConfig := *tlsConfig
			tlsConfig = &copyConfig
		}
		if tlsConfig.ServerName == "" {
			tlsConfig.ServerName = uri.Hostname()
		}
		return tls.Client(newCancelableConn(ctx, conn), tlsConfig), nil
	case "unix":
		address := uri.Host
		if address == "" {
			address = uri.Path
		}
		conn, err := dialer.DialContext(ctx, "unix", address)
		if err != nil {
			return nil, err
		}
		return newCancelableConn(ctx, conn), nil
	case "ws", "wss":
		wsOptions := options.WebsocketOptions
		wsDialer := websocket.Dialer{
			HandshakeTimeout:  options.ConnectTimeout,
			EnableCompression: false,
			Subprotocols:      []string{"mqtt"},
			NetDialContext:    dialer.DialContext,
		}
		if uri.Scheme == "wss" {
			tlsConfig := options.TLSConfig
			if tlsConfig == nil {
				tlsConfig = &tls.Config{}
			} else {
				copyConfig := *tlsConfig
				tlsConfig = &copyConfig
			}
			if tlsConfig.ServerName == "" {
				tlsConfig.ServerName = uri.Hostname()
			}
			wsDialer.TLSClientConfig = tlsConfig
		}
		if wsOptions != nil {
			wsDialer.ReadBufferSize = wsOptions.ReadBufferSize
			wsDialer.WriteBufferSize = wsOptions.WriteBufferSize
			if wsOptions.Proxy != nil {
				wsDialer.Proxy = func(req *http.Request) (*url.URL, error) {
					return wsOptions.Proxy(req)
				}
			}
		}
		ws, _, err := wsDialer.DialContext(ctx, uri.String(), options.HTTPHeaders)
		if err != nil {
			return nil, err
		}
		return newCancelableConn(ctx, &mqttWebsocketConn{conn: ws}), nil
	default:
		return nil, fmt.Errorf("unsupported MQTT broker URL scheme %q", uri.Scheme)
	}
}

// mqttWebsocketConn mirrors the small net.Conn adapter used by Paho while
// keeping the opener in this package cancellable through gorilla's DialContext.
type mqttWebsocketConn struct {
	conn *websocket.Conn
	r    io.Reader
	rio  sync.Mutex
	wio  sync.Mutex
}

func (c *mqttWebsocketConn) Read(p []byte) (int, error) {
	c.rio.Lock()
	defer c.rio.Unlock()
	for {
		if c.r == nil {
			_, reader, err := c.conn.NextReader()
			if err != nil {
				return 0, err
			}
			c.r = reader
		}
		n, err := c.r.Read(p)
		if err == io.EOF {
			c.r = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *mqttWebsocketConn) Write(p []byte) (int, error) {
	c.wio.Lock()
	defer c.wio.Unlock()
	if err := c.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *mqttWebsocketConn) Close() error         { return c.conn.Close() }
func (c *mqttWebsocketConn) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *mqttWebsocketConn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }
func (c *mqttWebsocketConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.conn.SetWriteDeadline(t)
}
func (c *mqttWebsocketConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *mqttWebsocketConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
