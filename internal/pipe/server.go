// Package pipe implements the pipe logic, i.e. listening for TLS or UDP
// connections and proxying data to the target destination.
package pipe

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AdguardTeam/golibs/log"
	"github.com/ameshkov/udptlspipe/internal/tunnel"
	"github.com/ameshkov/udptlspipe/internal/udp"
	"github.com/gobwas/ws"
	tls "github.com/refraction-networking/utls"
	"golang.org/x/net/proxy"
)

// defaultSNI is the default server name that will be used in both the client
// TLS ClientHello and the server's certificate when no TLS configuration is
// configured.
const defaultSNI = "example.org"

// upgradeTimeout is the read timeout for the first auth packet.
const upgradeTimeout = time.Second * 60

// Server represents an udptlspipe pipe. Depending on whether it is created in
// server- or client- mode, it listens to TLS or UDP connections and pipes the
// data to the destination.
type Server struct {
	listenAddr      string
	destinationAddr string
	dialer          proxy.Dialer
	serverMode      bool

	// tlsConfig to use for TLS connections. In server mode it also has the
	// certificate that will be used.
	tlsConfig *tls.Config

	// password is a string that the server will search for in the first bytes.
	// If not found, the server will return a stub web page.
	password string

	// listen is the TLS listener for incoming connections
	listen net.Listener

	// srcConns is a set that is used to track active incoming TCP connections.
	srcConns   map[net.Conn]struct{}
	srcConnsMu *sync.Mutex

	// dstConns is a set that is used to track active connections to the proxy
	// destination.
	dstConns   map[net.Conn]struct{}
	dstConnsMu *sync.Mutex

	// Shutdown handling
	// --

	// lock protects started, tcpListener and udpListener.
	lock    sync.RWMutex
	started bool
	// wg tracks active workers. Stop won't finish until there is at least
	// won't finish until there's at least one active worker.
	wg sync.WaitGroup
}

// Config represents the server configuration.
type Config struct {
	// ListenAddr is the address (ip:port) where the server will be listening
	// to. Depending on the mode the server uses, it will either listen for TLS
	// or UDP connections.
	ListenAddr string

	// DestinationAddr is the address (host:port) to where the server will try
	// to connect. Depending on the mode the server uses, it will either
	// connect to a TLS endpoint (the pipe server) or not.
	DestinationAddr string

	// Password enables authentication of the pipe clients. If set, it also
	// enables active probing protection.
	Password string

	// ServerMode controls the way the pipe operates. When it's true, the pipe
	// server operates in server mode, i.e. it accepts incoming TLS connections
	// and proxies the data to the destination address over UDP. When it works
	// in client mode, it is the other way around: accepts UDP traffic and
	// proxies it to the destination pipe server over TLS.
	ServerMode bool

	// URL of a proxy server that can be used for proxying traffic to the
	// destination.
	ProxyURL string

	// VerifyCertificate enables server certificate verification in client mode.
	// If enabled, the client will verify the server certificate using the
	// system root certs store.
	VerifyCertificate bool

	// TLSServerName configures the server name to send in TLS ClientHello when
	// operating in client mode and the server name that will be used when
	// generating a stub certificate. If not set, the default domain name will
	// be used for these purposes.
	TLSServerName string

	// TLSCertificate is an optional field that allows to configure the TLS
	// certificate to use when running in server mode. This option makes sense
	// only for server mode. If not configured, the server will generate a stub
	// self-signed certificate automatically.
	TLSCertificate *tls.Certificate
}

// createTLSConfig creates a TLS configuration as per the server configuration.
func createTLSConfig(config *Config) (tlsConfig *tls.Config, err error) {
	serverName := config.TLSServerName
	if serverName == "" {
		log.Info("TLS server name is not configured, using %s by default", defaultSNI)
		serverName = defaultSNI
	}

	if config.ServerMode {
		tlsCert := config.TLSCertificate
		if tlsCert == nil {
			log.Info("Generating a stub certificate for %s", serverName)
			tlsCert, err = createStubCertificate(serverName)
			if err != nil {
				return nil, fmt.Errorf("failed to generate a stub certificate: %w", err)
			}
		} else {
			log.Info("Using the supplied TLS certificate")
		}

		tlsConfig = &tls.Config{
			ServerName:   serverName,
			Certificates: []tls.Certificate{*tlsCert},
			MinVersion:   tls.VersionTLS12,
		}
	} else {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: !config.VerifyCertificate,
			ServerName:         serverName,
		}
	}

	return tlsConfig, nil
}

// NewServer creates a new instance of a *Server.
func NewServer(config *Config) (s *Server, err error) {
	s = &Server{
		listenAddr:      config.ListenAddr,
		destinationAddr: config.DestinationAddr,
		password:        config.Password,
		dialer:          proxy.Direct,
		serverMode:      config.ServerMode,
		srcConns:        map[net.Conn]struct{}{},
		srcConnsMu:      &sync.Mutex{},
		dstConns:        map[net.Conn]struct{}{},
		dstConnsMu:      &sync.Mutex{},
	}

	s.tlsConfig, err = createTLSConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare TLS configuration: %w", err)
	}

	if config.ProxyURL != "" {
		var u *url.URL
		u, err = url.Parse(config.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}

		s.dialer, err = proxy.FromURL(u, s.dialer)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize proxy dialer: %w", err)
		}
	}

	return s, nil
}

// Addr returns the address the pipe listens to if it is started or nil.
func (s *Server) Addr() (addr net.Addr) {
	if s.listen == nil {
		return nil
	}

	return s.listen.Addr()
}

// Start starts the pipe, exits immediately if it failed to start
// listening.  Start returns once all servers are considered up.
func (s *Server) Start() (err error) {
	log.Info("Starting the pipe %s", s)

	s.lock.Lock()
	defer s.lock.Unlock()

	if s.started {
		return errors.New("pipe is already started")
	}

	s.listen, err = s.createListener()
	if err != nil {
		return fmt.Errorf("failed to start pipe: %w", err)
	}

	s.wg.Add(1)
	go s.serve()

	s.started = true
	log.Info("Server has been started")

	return nil
}

// createListener creates a TLS listener in server mode and UDP listener in
// client mode.
func (s *Server) createListener() (l net.Listener, err error) {
	if s.serverMode {
		l, err = tls.Listen("tcp", s.listenAddr, s.tlsConfig)
		if err != nil {
			return nil, err
		}
	} else {
		l, err = udp.Listen("udp", s.listenAddr)
		if err != nil {
			return nil, err
		}
	}

	return l, nil
}

// Shutdown stops the pipe and waits for all active connections to close.
func (s *Server) Shutdown(ctx context.Context) (err error) {
	log.Info("Stopping the pipe %s", s)

	s.stopServeLoop()

	// Closing the udpConn thread.
	log.OnCloserError(s.listen, log.DEBUG)

	// Closing active TCP connections.
	s.closeConnections(s.srcConnsMu, s.srcConns)

	// Closing active UDP connections.
	s.closeConnections(s.dstConnsMu, s.dstConns)

	// Wait until all worker threads finish working
	err = s.waitShutdown(ctx)

	log.Info("Server has been stopped")

	return err
}

// closeConnections closes all active connections.
func (s *Server) closeConnections(mu *sync.Mutex, conns map[net.Conn]struct{}) {
	mu.Lock()
	defer mu.Unlock()

	for c := range conns {
		_ = c.SetReadDeadline(time.Unix(1, 0))

		log.OnCloserError(c, log.DEBUG)
	}
}

// stopServeLoop sets the started flag to false thus stopping the serving loop.
func (s *Server) stopServeLoop() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.started = false
}

// type check
var _ fmt.Stringer = (*Server)(nil)

// String implements the fmt.Stringer interface for *Server.
func (s *Server) String() (str string) {
	switch s.serverMode {
	case true:
		return fmt.Sprintf("tls://%s <-> udp://%s", s.listenAddr, s.destinationAddr)
	default:
		return fmt.Sprintf("udp://%s <-> tls://%s", s.listenAddr, s.destinationAddr)
	}
}

// serve implements the pipe logic, i.e. accepts new connections and tunnels
// data to the destination.
func (s *Server) serve() {
	defer s.wg.Done()
	defer log.OnPanicAndExit("serve", 1)

	defer log.OnCloserError(s.listen, log.DEBUG)

	for s.isStarted() {
		err := s.acceptConn()
		if err != nil {
			if !s.isStarted() {
				return
			}

			log.Error("exit serve loop due to: %v", err)

			return
		}
	}
}

// acceptConn accepts new incoming and tracks active connections.
func (s *Server) acceptConn() (err error) {
	conn, err := s.listen.Accept()
	if err != nil {
		// This type of errors should not lead to stopping the pipe.
		if errors.Is(os.ErrDeadlineExceeded, err) {
			return nil
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil
		}

		return err
	}

	log.Debug("Accepted new connection from %s", conn.RemoteAddr())

	func() {
		s.srcConnsMu.Lock()
		defer s.srcConnsMu.Unlock()

		// Track the connection to allow unblocking reads on shutdown.
		s.srcConns[conn] = struct{}{}
	}()

	s.wg.Add(1)
	go s.serveConn(conn)

	return nil
}

// closeSrcConn closes the source connection and cleans up after it.
func (s *Server) closeSrcConn(conn net.Conn) {
	log.OnCloserError(conn, log.DEBUG)

	s.srcConnsMu.Lock()
	defer s.srcConnsMu.Unlock()

	delete(s.srcConns, conn)
}

// closeDstConn closes the destination connection and cleans up after it.
func (s *Server) closeDstConn(conn net.Conn) {
	// No destination connection opened yet, do nothing.
	if conn != nil {
		return
	}

	log.OnCloserError(conn, log.DEBUG)

	s.dstConnsMu.Lock()
	defer s.dstConnsMu.Unlock()

	delete(s.dstConns, conn)
}

// readWriteCloser is a helper object that's used for replacing
// io.ReadWriteCloser when the server peeked into the connection.
type readWriteCloser struct {
	io.Reader
	io.Writer
	io.Closer
}

// upgradeClientConn
func (s *Server) upgradeClientConn(conn net.Conn) (rwc io.ReadWriteCloser, err error) {
	log.Debug("Upgrading connection to %s", conn.RemoteAddr())

	// Give up to 60 seconds on the upgrade and authentication.
	_ = conn.SetReadDeadline(time.Now().Add(upgradeTimeout))
	defer func() {
		// Remove the deadline when it's not required any more.
		_ = conn.SetReadDeadline(time.Time{})
	}()

	u, err := url.Parse(fmt.Sprintf("wss://%s/?password=%s", s.tlsConfig.ServerName, s.password))
	if err != nil {
		return nil, err
	}

	var br *bufio.Reader
	br, _, err = ws.DefaultDialer.Upgrade(conn, u)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade: %w", err)
	}

	if br != nil && br.Buffered() > 0 {
		// If Upgrade returned a non-empty reader, then probably the server
		// immediately sent some data. This is not the expected behavior so
		// raise an error here.
		return nil, fmt.Errorf("received initial data len=%d from the server", br.Buffered())
	}

	return newWsConn(
		&readWriteCloser{
			Reader: conn,
			Writer: conn,
			Closer: conn,
		},
		conn.RemoteAddr(),
		ws.StateClientSide,
	), nil
}

// respondWithDummyPage writes a dummy response to the client (part of active
// probing protection).
func (s *Server) respondWithDummyPage(conn net.Conn, req *http.Request) {
	response := fmt.Sprintf("%s 403 Forbidden\r\n", req.Proto) +
		"Server: nginx\r\n" +
		fmt.Sprintf("Date: %s\r\n", time.Now().Format(http.TimeFormat)) +
		"Content-Type: text/html\r\n" +
		"Connection: close\r\n" +
		"\r\n" +
		"<html>\r\n" +
		"<head><title>403 Forbidden</title></head>\r\n" +
		"<center><h1>403 Forbidden</h1></center>\r\n" +
		"<hr><center>nginx</center>\r\n" +
		"</body>\r\n" +
		"</html>\r\n"

	_, _ = conn.Write([]byte(response))
}

// upgradeServerConn attempts to upgrade the server connection and returns a
// rwc that wraps the original connection and can be used for tunneling data.
func (s *Server) upgradeServerConn(conn net.Conn) (rwc io.ReadWriteCloser, err error) {
	log.Debug("Upgrading connection from %s", conn.RemoteAddr())

	// Give up to 60 seconds on the upgrade and authentication.
	_ = conn.SetReadDeadline(time.Now().Add(upgradeTimeout))
	defer func() {
		// Remove the deadline when it's not required any more.
		_ = conn.SetReadDeadline(time.Time{})
	}()

	// bufio.Reader may read more than requested, so it's crucial to use
	// TeeReader so that we could restore the bytes that has been read.
	var buf bytes.Buffer
	r := bufio.NewReader(io.TeeReader(conn, &buf))

	req, err := http.ReadRequest(r)
	if err != nil {
		return nil, fmt.Errorf("cannot read HTTP request: %w", err)
	}

	if !strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
		s.respondWithDummyPage(conn, req)

		return nil, fmt.Errorf("not a websocket")
	}

	clientPassword := req.URL.Query().Get("password")
	if s.password != "" && clientPassword != s.password {
		s.respondWithDummyPage(conn, req)

		return nil, fmt.Errorf("wrong password: %s", clientPassword)
	}

	// Now that authentication check has been done restore the peeked up data
	// and use ws.Upgrade to do the actual WebSocket upgrade.
	multiRwc := &readWriteCloser{
		Reader: io.MultiReader(bytes.NewReader(buf.Bytes()), conn),
		Writer: conn,
		Closer: conn,
	}

	_, err = ws.Upgrade(multiRwc)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade WebSocket: %w", err)
	}

	return newWsConn(multiRwc, conn.RemoteAddr(), ws.StateServerSide), nil
}

// serveConn processes incoming connection, authenticates it and proxies the
// data from it to the destination address.
func (s *Server) serveConn(conn net.Conn) {
	defer func() {
		s.wg.Done()

		s.closeSrcConn(conn)
	}()

	var rwc io.ReadWriteCloser = conn

	if s.serverMode {
		var err error
		rwc, err = s.upgradeServerConn(conn)
		if err != nil {
			log.Error("failed to accept server conn: %v", err)

			return
		}
	}

	s.processConn(rwc)
}

// processConn processes the prepared server connection that is passed as rwc.
func (s *Server) processConn(rwc io.ReadWriteCloser) {
	var dstConn net.Conn

	defer s.closeDstConn(dstConn)

	dstConn, err := s.dialDst()
	if err != nil {
		log.Error("failed to connect to %s: %v", s.destinationAddr, err)

		return
	}

	func() {
		s.dstConnsMu.Lock()
		defer s.dstConnsMu.Unlock()

		// Track the connection to allow unblocking reads on shutdown.
		s.dstConns[dstConn] = struct{}{}
	}()

	var dstRwc io.ReadWriteCloser = dstConn
	if !s.serverMode {
		dstRwc, err = s.upgradeClientConn(dstConn)
		if err != nil {
			log.Error("failed to upgrade: %v", err)
		}
	}

	// Prepare ReadWriter objects for tunneling.
	var srcRw, dstRw io.ReadWriter
	srcRw = rwc
	dstRw = dstRwc

	// When the client communicates with the server it uses encoded messages so
	// connection between them needs to be wrapped. In server mode it is the
	// source connection, in client mode it is the destination connection.
	if s.serverMode {
		srcRw = tunnel.NewMsgReadWriter(srcRw)
	} else {
		dstRw = tunnel.NewMsgReadWriter(dstRw)
	}

	tunnel.Tunnel(s.String(), srcRw, dstRw)
}

// dialDst creates a connection to the destination. Depending on the mode the
// server operates in, it is either a TLS connection or a UDP connection.
func (s *Server) dialDst() (conn net.Conn, err error) {
	if s.serverMode {
		return s.dialer.Dial("udp", s.destinationAddr)
	}

	tcpConn, err := s.dialer.Dial("tcp", s.destinationAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to %s: %w", s.destinationAddr, err)
	}

	tlsConn := tls.UClient(tcpConn, s.tlsConfig, tls.HelloAndroid_11_OkHttp)

	err = tlsConn.Handshake()
	if err != nil {
		return nil, fmt.Errorf("cannot establish connection to %s: %w", s.destinationAddr, err)
	}

	return tlsConn, nil
}

// isStarted safely checks whether the pipe is started or not.
func (s *Server) isStarted() (started bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.started
}

// waitShutdown waits either until context deadline OR Server.wg.
func (s *Server) waitShutdown(ctx context.Context) (err error) {
	// Using this channel to wait until all goroutines finish their work.
	closed := make(chan struct{})
	go func() {
		defer log.OnPanic("waitShutdown")

		// Wait until all active workers finished its work.
		s.wg.Wait()
		close(closed)
	}()

	var ctxErr error
	select {
	case <-closed:
		// Do nothing here.
	case <-ctx.Done():
		ctxErr = ctx.Err()
	}

	return ctxErr
}
