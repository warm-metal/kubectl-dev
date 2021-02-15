package session

import (
	"io"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

type clientReader struct {
	s      AppGate_OpenAppServer
	size   remotecommand.TerminalSize
	stdin  chan string
	closed bool
}

func (r *clientReader) Close() {
	r.closed = true
}

func (r *clientReader) loop() {
	r.stdin = make(chan string)
	defer func() {
		r.closed = true
		close(r.stdin)
	}()

	for {
		if r.closed {
			return
		}

		req, err := r.s.Recv()
		if err != nil {
			klog.Errorf("can't read stdin: %s", err)
			return
		}

		if req.TerminalSize != nil {
			r.size.Width = uint16(req.TerminalSize.Width)
			r.size.Height = uint16(req.TerminalSize.Height)
		}

		if len(req.Stdin) > 0 {
			if len(req.Stdin) != 1 {
				klog.Errorf("invalid input %#v", req.Stdin)
				return
			}
			r.stdin <- req.Stdin[0]
		}
	}
}

func (r clientReader) Next() *remotecommand.TerminalSize {
	if r.closed {
		return nil
	}

	return &r.size
}

func (r *clientReader) Read(p []byte) (n int, err error) {
	in, ok := <-r.stdin
	if !ok {
		err = io.EOF
		return
	}

	if len(p) < len(in) {
		err = io.ErrShortBuffer
		klog.Errorf("buffer too small %d, %d", len(p), len(in))
		return
	}

	n = copy(p, in)
	return
}

type stdoutWriter struct {
	s AppGate_OpenAppServer
}

func (w stdoutWriter) Write(p []byte) (n int, err error) {
	err = w.s.Send(&AppResponse{
		Stdout: string(p),
	})

	if err != nil {
		klog.Errorf("can't write stdout: %s", err)
		return
	}

	n = len(p)
	return
}

func genIOStreams(s AppGate_OpenAppServer, initSize *TerminalSize) (reader *clientReader, stdout io.Writer) {
	in := clientReader{s: s, size: remotecommand.TerminalSize{
		Width:  uint16(initSize.Width),
		Height: uint16(initSize.Height),
	}}
	go in.loop()
	return &in, &stdoutWriter{s}
}
