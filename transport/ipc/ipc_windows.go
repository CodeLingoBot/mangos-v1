// +build windows

// Copyright 2018 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ipc implements the IPC transport on top of Windows Named Pipes.
package ipc

import (
	"net"

	"github.com/Microsoft/go-winio"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/transport"
)

// The options here are pretty specific to Windows Named Pipes.

const (
	// OptionSecurityDescriptor represents a Windows security
	// descriptor in SDDL format (string).  This can only be set on
	// a Listener, and must be set before the Listen routine
	// is called.
	OptionSecurityDescriptor = "WIN-IPC-SECURITY-DESCRIPTOR"

	// OptionInputBufferSize represents the Windows Named Pipe
	// input buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	OptionInputBufferSize = "WIN-IPC-INPUT-BUFFER-SIZE"

	// OptionOutputBufferSize represents the Windows Named Pipe
	// output buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	OptionOutputBufferSize = "WIN-IPC-OUTPUT-BUFFER-SIZE"
)

type pipeAddr string

func (p pipeAddr) Network() string {
	return "ipc"
}

func (p pipeAddr) String() string {
	return string(p)
}

type dialer struct {
	path  string
	proto transport.ProtocolInfo
	opts  map[string]interface{}
}

// Dial implements the PipeDialer Dial method.
func (d *dialer) Dial() (transport.Pipe, error) {

	conn, err := winio.DialPipe("\\\\.\\pipe\\"+d.path, nil)
	if err != nil {
		return nil, err
	}
	addr := pipeAddr(d.path)
	return mangos.NewConnPipeIPC(conn, d.proto, d.opts)
}

// SetOption implements a stub PipeDialer SetOption method.
func (d *dialer) SetOption(n string, v interface{}) error {
	return mangos.ErrBadOption
}

// GetOption implements a stub PipeDialer GetOption method.
func (d *dialer) GetOption(n string) (interface{}, error) {
	return nil, mangos.ErrBadOption
}

// listenerOptions is used for shared GetOption/SetOption logic for listeners.
// We don't have dialer specific options at this point.
type listenerOptions map[string]interface{}

// GetOption retrieves an option value.
func (o listenerOptions) get(name string) (interface{}, error) {
	if o == nil {
		return nil, mangos.ErrBadOption
	}
	v, ok := o[name]
	if !ok {
		return nil, mangos.ErrBadOption
	}
	return v, nil
}

// SetOption sets an option.  We have none, so just ErrBadOption.
func (o listenerOptions) set(string, interface{}) error {
	return mangos.ErrBadOption
}

type listener struct {
	path     string
	sock     mangos.Socket
	listener net.Listener
}

// Listen implements the PipeListener Listen method.
func (l *listener) Listen() error {

	config := &winio.PipeConfig{
		InputBufferSize:    l.opts[OptionInputBufferSize].(int32),
		OutputBufferSize:   l.opts[OptionOutputBufferSize].(int32),
		SecurityDescriptor: l.opts[OptionSecurityDescriptor].(string),
		MessageMode:        false,
	}

	listener, err := winio.ListenPipe("\\\\.\\pipe\\"+l.path, config)
	if err != nil {
		return err
	}
	l.listener = listener
	return nil
}

func (l *listener) Address() string {
	return "ipc://" + l.path
}

// Accept implements the the PipeListener Accept method.
func (l *listener) Accept() (mangos.TranPipe, error) {

	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	addr := pipeAddr(l.path)
	return transport.NewConnPipeIPC(conn, l.proto, l.opts)
}

// Close implements the PipeListener Close method.
func (l *listener) Close() error {
	if l.listener != nil {
		l.listener.Close()
	}
	return nil
}

// SetOption implements a stub PipeListener SetOption method.
func (l *listener) SetOption(name string, val interface{}) error {
	switch name {
	case OptionInputBufferSize:
		fallthrough
	case OptionOutputBufferSize:
		if v, ok := val.(int32); ok {
			l.opts[name] = v
			return nil
		}
		return mangos.ErrBadValue

	case OptionSecurityDescriptor:
		if v, ok := val.(string); ok {
			l.opts[name] = v
			return nil
		}
		return mangos.ErrBadValue

	case mangos.OptionMaxRecvSize:
		if v, ok := val.(int64); ok {
			l.opts[name] = v
			return nil
		}
		return mangos.ErrBadValue
	default:
		return mangos.ErrBadOption
	}
}

// GetOption implements a stub PipeListener GetOption method.
func (l *listener) GetOption(name string) (interface{}, error) {
	if v, ok := l.opts[name]; ok {
		return v, nil
	}
	return nil, mangos.ErrBadOption
}

type ipcTran struct{}

// Scheme implements the Transport Scheme method.
func (t *ipcTran) Scheme() string {
	return "ipc"
}

// NewDialer implements the Transport NewDialer method.
func (t *ipcTran) NewDialer(addr string, sock mangos.Socket) (mangos.TranDialer, error) {
	var err error

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	d := &dialer{
		proto: sock.Info(),
		path:  addr,
		opts:  make(map[string]interface{}),
	}

	d.opts[mangos.OptionLocalAddr] = addr
	d.opts[mangos.OptionRemoteAddr] = addr
	d.opts[mangos.OptionMaxRecvSize] = int64(0)

	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t *ipcTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	var err error

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	l := &listener{
		proto: sock.Info(),
		path:  addr,
		opts:  make(map[string]interface{}),
	}

	l.opts[OptionInputBufferSize] = int32(4096)
	l.opts[OptionOutputBufferSize] = int32(4096)
	l.opts[OptionSecurityDescriptor] = ""
	l.opts[mangos.OptionLocalAddr] = addr
	l.opts[mangos.OptionRemoteAddr] = addr
	l.opts[mangos.OptionMaxRecvSize] = int64(0)

	return l, nil
}

// NewTransport allocates a new IPC transport.
func NewTransport() transport.Transport {
	return &ipcTran{}
}
