package lizard

import (
	"context"
	"net"
	"sync"
	"time"
)

type agentManager struct {
	agents []*agent
}

type agent struct {
	address string
	conn    net.Conn
}

type GatewayConn struct {
	netid   int64
	belong  *Gateway
	rawConn net.Conn

	once      *sync.Once
	wg        *sync.WaitGroup
	sendCh    chan []byte
	handlerCh chan MessageHandler
	timerCh   chan *OnTimeOut

	mu      sync.Mutex // guards following
	name    string
	heart   int64
	pending []int64
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewServerConn returns a new server connection which has not started to
// serve requests yet.
func NewGatewayConn(id int64, s *Gateway, c net.Conn) *GatewayConn {
	sc := &GatewayConn{
		netid:     id,
		belong:    s,
		rawConn:   c,
		once:      &sync.Once{},
		wg:        &sync.WaitGroup{},
		sendCh:    make(chan []byte, s.opts.bufferSize),
		handlerCh: make(chan MessageHandler, s.opts.bufferSize),
		timerCh:   make(chan *OnTimeOut, s.opts.bufferSize),
		heart:     time.Now().UnixNano(),
	}
	sc.ctx, sc.cancel = context.WithCancel(context.WithValue(s.ctx, serverCtx, s))
	sc.name = c.RemoteAddr().String()
	sc.pending = []int64{}
	return sc
}
