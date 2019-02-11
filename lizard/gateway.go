package lizard

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/leesper/holmes"
)

// func init() {
// 	netIdentifier = NewAtomicInt64(0)
// }

// var (
// 	netIdentifier *AtomicInt64
// 	tlsWrapper    func(net.Conn) net.Conn
// )

// type options struct {
// 	tlsCfg     *tls.Config
// 	codec      Codec
// 	onConnect  onConnectFunc
// 	onMessage  onMessageFunc
// 	onClose    onCloseFunc
// 	onError    onErrorFunc
// 	workerSize int  // numbers of worker go-routines
// 	bufferSize int  // size of buffered channel
// 	reconnect  bool // for ClientConn use only
// }

// ServerOption sets server options.
//type ServerOption func(*options)

// ReconnectOption returns a ServerOption that will make ClientConn reconnectable.
// func ReconnectOption() ServerOption {
// 	return func(o *options) {
// 		o.reconnect = true
// 	}
// }

// CustomCodecOption returns a ServerOption that will apply a custom Codec.
// func CustomCodecOption(codec Codec) ServerOption {
// 	return func(o *options) {
// 		o.codec = codec
// 	}
// }

// TLSCredsOption returns a ServerOption that will set TLS credentials for server
// connections.
// func TLSCredsOption(config *tls.Config) ServerOption {
// 	return func(o *options) {
// 		o.tlsCfg = config
// 	}
// }

// WorkerSizeOption returns a ServerOption that will set the number of go-routines
// in WorkerPool.
// func WorkerSizeOption(workerSz int) ServerOption {
// 	return func(o *options) {
// 		o.workerSize = workerSz
// 	}
// }

// BufferSizeOption returns a ServerOption that is the size of buffered channel,
// for example an indicator of BufferSize256 means a size of 256.
// func BufferSizeOption(indicator int) ServerOption {
// 	return func(o *options) {
// 		o.bufferSize = indicator
// 	}
// }

// OnConnectOption returns a ServerOption that will set callback to call when new
// client connected.
// func OnConnectOption(cb func(WriteCloser) bool) ServerOption {
// 	return func(o *options) {
// 		o.onConnect = cb
// 	}
// }

// OnMessageOption returns a ServerOption that will set callback to call when new
// message arrived.
// func OnMessageOption(cb func(Message, WriteCloser)) ServerOption {
// 	return func(o *options) {
// 		o.onMessage = cb
// 	}
// }

// OnCloseOption returns a ServerOption that will set callback to call when client
// closed.
// func OnCloseOption(cb func(WriteCloser)) ServerOption {
// 	return func(o *options) {
// 		o.onClose = cb
// 	}
// }

// OnErrorOption returns a ServerOption that will set callback to call when error
// occurs.
// func OnErrorOption(cb func(WriteCloser)) ServerOption {
// 	return func(o *options) {
// 		o.onError = cb
// 	}
// }

// Gateway  is a server to serve TCP requests.
type Gateway struct {
	opts   options
	ctx    context.Context
	cancel context.CancelFunc
	conns  *sync.Map
	timing *TimingWheel
	wg     *sync.WaitGroup
	mu     sync.Mutex // guards following
	lis    map[net.Listener]bool
	// for periodically running function every duration.
	interv time.Duration
	sched  onScheduleFunc
	proxy  map[int32]agentManager
}

// NewGateway returns a new TCP server which has not started
// to serve requests yet.
func NewGateway(opt ...ServerOption) *Gateway {
	var opts options
	for _, o := range opt {
		o(&opts)
	}

	if opts.codec == nil {
		//TODO opts.codec = TypeLengthValueCodec{}
	}
	if opts.workerSize <= 0 {
		opts.workerSize = defaultWorkersNum
	}
	if opts.bufferSize <= 0 {
		opts.bufferSize = BufferSize256
	}

	// initiates go-routine pool instance
	globalWorkerPool = newWorkerPool(opts.workerSize)

	s := &Gateway{
		opts:  opts,
		conns: &sync.Map{},
		wg:    &sync.WaitGroup{},
		lis:   make(map[net.Listener]bool),
		proxy: make(map[int32]agentManager),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.timing = NewTimingWheel(s.ctx)
	return s
}

// ConnsSize returns connections size.
func (s *Gateway) ConnsSize() int {
	var sz int
	s.conns.Range(func(k, v interface{}) bool {
		sz++
		return true
	})
	return sz
}

// Sched sets a callback to invoke every duration.
func (s *Gateway) Sched(dur time.Duration, sched func(time.Time, WriteCloser)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interv = dur
	s.sched = onScheduleFunc(sched)
}

// Broadcast broadcasts message to all server connections managed.
func (s *Gateway) Broadcast(msg ZMessage) {
	s.conns.Range(func(k, v interface{}) bool {
		c := v.(*ServerConn)
		if err := c.Write(msg); err != nil {
			holmes.Errorf("broadcast error %v, conn id %d", err, k.(int64))
			return false
		}
		return true
	})
}

// Unicast unicasts message to a specified conn.
func (s *Gateway) Unicast(id int64, msg ZMessage) error {
	v, ok := s.conns.Load(id)
	if ok {
		return v.(*ServerConn).Write(msg)
	}
	return fmt.Errorf("conn id %d not found", id)
}

// Conn returns a server connection with specified ID.
func (s *Gateway) Conn(id int64) (*ServerConn, bool) {
	v, ok := s.conns.Load(id)
	if ok {
		return v.(*ServerConn), ok
	}
	return nil, ok
}

// Start starts the TCP server, accepting new clients and creating service
// go-routine for each. The service go-routines read messages and then call
// the registered handlers to handle them. Start returns when failed with fatal
// errors, the listener willl be closed when returned.
func (s *Gateway) Start(l net.Listener) error {
	s.mu.Lock()
	if s.lis == nil {
		s.mu.Unlock()
		l.Close()
		return ErrServerClosed
	}
	s.lis[l] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.lis != nil && s.lis[l] {
			l.Close()
			delete(s.lis, l)
		}
		s.mu.Unlock()
	}()

	holmes.Infof("server start, net %s addr %s\n", l.Addr().Network(), l.Addr().String())

	s.wg.Add(1)
	go s.timeOutLoop()

	var tempDelay time.Duration
	for {
		rawConn, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay >= max {
					tempDelay = max
				}
				holmes.Errorf("accept error %v, retrying in %d\n", err, tempDelay)
				select {
				case <-time.After(tempDelay):
				case <-s.ctx.Done():
				}
				continue
			}
			return err
		}
		tempDelay = 0

		// how many connections do we have ?
		sz := s.ConnsSize()
		if sz >= MaxConnections {
			holmes.Warnf("max connections size %d, refuse\n", sz)
			rawConn.Close()
			continue
		}

		if s.opts.tlsCfg != nil {
			rawConn = tls.Server(rawConn, s.opts.tlsCfg)
		}

		netid := netIdentifier.GetAndIncrement()
		sc := NewGatewayConn(netid, s, rawConn)
		fmt.Println("====>>>gateway netid:", netid)
		sc.SetName(sc.rawConn.RemoteAddr().String())

		s.mu.Lock()
		if s.sched != nil {
			//TODO sc.RunEvery(s.interv, s.sched)
		}
		s.mu.Unlock()

		s.conns.Store(netid, sc)
		addTotalConn(1)

		s.wg.Add(1) // this will be Done() in ServerConn.Close()
		go func() {
			sc.Start()
		}()

		//TODO holmes.Infof("accepted client %s, id %d, total %d\n", sc.Name(), netid, s.ConnsSize())
		s.conns.Range(func(k, v interface{}) bool {
			i := k.(int64)
			c := v.(*GatewayConn)
			holmes.Infof("client(%d) %s", i, c.Name())
			return true
		})
	} // for loop
}

// Name returns the name of server connection.
func (sc *GatewayConn) Name() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	name := sc.name
	return name
}

// Stop gracefully closes the server, it blocked until all connections
// are closed and all go-routines are exited.
func (s *Gateway) Stop() {
	// immediately stop accepting new clients
	s.mu.Lock()
	listeners := s.lis
	s.lis = nil
	s.mu.Unlock()

	for l := range listeners {
		l.Close()
		holmes.Infof("stop accepting at address %s\n", l.Addr().String())
	}

	// close all connections
	conns := map[int64]*ServerConn{}

	s.conns.Range(func(k, v interface{}) bool {
		i := k.(int64)
		c := v.(*ServerConn)
		conns[i] = c
		return true
	})
	// let GC do the cleanings
	s.conns = nil

	for _, c := range conns {
		c.rawConn.Close()
		holmes.Infof("close client %s\n", c.Name())
	}

	s.mu.Lock()
	s.cancel()
	s.mu.Unlock()

	s.wg.Wait()

	holmes.Infoln("server stopped gracefully, bye.")
	os.Exit(0)
}

// Retrieve the extra data(i.e. net id), and then redispatch timeout callbacks
// to corresponding client connection, this prevents one client from running
// callbacks of other clients
func (s *Gateway) timeOutLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return

		case timeout := <-s.timing.TimeOutChannel():
			netID := timeout.Ctx.Value(netIDCtx).(int64)
			if v, ok := s.conns.Load(netID); ok {
				sc := v.(*ServerConn)
				sc.timerCh <- timeout
			} else {
				holmes.Warnf("invalid client %d", netID)
			}
		}
	}
}
