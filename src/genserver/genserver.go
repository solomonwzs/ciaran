package genserver

type Server interface {
	Init(interface{}) error
	HandleCall(interface{}) (interface{}, error)
	HandleCast(interface{}) error
	Terminate(error)
}

type ServerPID struct {
	mailbox chan *message
	s       Server
	closed  bool
}

type Options struct {
	BufferSize uint
}

type message struct {
	data interface{}
	ch   chan interface{}
}

func Start(s Server, arg interface{}, opt *Options) (p *ServerPID, err error) {
	p = &ServerPID{
		mailbox: make(chan *message, opt.BufferSize),
		s:       s,
	}

	if err = s.Init(arg); err != nil {
		return nil, err
	}

	go func() {
		var (
			ret  interface{}
			err0 error
		)
		for msg := range p.mailbox {
			if msg.ch == nil {
				if err0 = p.s.HandleCast(msg.data); err != nil {
					break
				}
			} else {
				if ret, err0 = p.s.HandleCall(msg.data); err != nil {
					msg.ch <- nil
					break
				} else {
					msg.ch <- ret
				}
			}
		}
		p.s.Terminate(err0)
	}()

	return
}

func Close(p *ServerPID) {
	if !p.closed {
		p.closed = true
		close(p.mailbox)
	}
}

func Call(p *ServerPID, req interface{}) interface{} {
	msg := &message{
		data: req,
		ch:   make(chan interface{}),
	}
	p.mailbox <- msg
	return <-msg.ch
}

func Cast(p *ServerPID, req interface{}) {
	msg := &message{
		data: req,
		ch:   nil,
	}
	p.mailbox <- msg
}
