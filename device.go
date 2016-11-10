// Copyright 2016 The fer Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fer

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sbinet-alice/fer/config"
	"github.com/sbinet-alice/fer/mq"
	_ "github.com/sbinet-alice/fer/mq/nanomsg" // load nanomsg plugin
	_ "github.com/sbinet-alice/fer/mq/zeromq"  // load zeromq plugin
)

// FIXME(sbinet) use a per-device stdout
var stdout = bufio.NewWriter(os.Stdout)

type channel struct {
	cfg config.Channel
	sck mq.Socket
	cmd chan Cmd
	msg chan Msg
	log *log.Logger
}

func (ch *channel) Name() string {
	return ch.cfg.Name
}

func (ch *channel) Send(data []byte) (int, error) {
	err := ch.sck.Send(data)
	return len(data), err
}

func (ch *channel) Recv() ([]byte, error) {
	return ch.sck.Recv()
}

func (ch *channel) run(ctx context.Context) {
	for {
		select {
		case msg := <-ch.msg:
			_, err := ch.Send(msg.Data)
			if err != nil {
				ch.log.Fatalf("send error: %v\n", err)
			}
		case ch.msg <- ch.recv():
		case cmd := <-ch.cmd:
			switch cmd {
			case CmdEnd:
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (ch *channel) recv() Msg {
	data, err := ch.Recv()
	return Msg{
		Data: data,
		Err:  err,
	}
}

func newChannel(drv mq.Driver, cfg config.Channel) (channel, error) {
	ch := channel{
		cmd: make(chan Cmd),
		cfg: cfg,
		log: log.New(stdout, cfg.Name+": ", 0),
	}
	// FIXME(sbinet) support multiple sockets to send/recv to/from
	if len(cfg.Sockets) != 1 {
		panic("fer: not implemented")
	}
	typ := mq.SocketTypeFrom(cfg.Sockets[0].Type)
	sck, err := drv.NewSocket(typ)
	if err != nil {
		return ch, err
	}
	ch.sck = sck
	return ch, nil
}

type device struct {
	name  string
	chans map[string][]channel
	cmds  chan Cmd
	msgs  map[msgAddr]chan Msg
	msg   *log.Logger
}

func newDevice(drv mq.Driver, cfg config.Device) (*device, error) {
	msg := log.New(stdout, cfg.Name()+": ", 0)
	msg.Printf("--- new device: %v\n", cfg)
	dev := device{
		chans: make(map[string][]channel),
		cmds:  make(chan Cmd),
		msgs:  make(map[msgAddr]chan Msg),
		msg:   msg,
	}

	for _, opt := range cfg.Channels {
		dev.msg.Printf("--- new channel: %v\n", opt)
		ch, err := newChannel(drv, opt)
		if err != nil {
			return nil, err
		}
		ch.msg = make(chan Msg)
		dev.chans[opt.Name] = []channel{ch}
		dev.msgs[msgAddr{name: opt.Name, id: 0}] = ch.msg
	}
	return &dev, nil
}

func (dev *device) Chan(name string, i int) (chan Msg, error) {
	msg, ok := dev.msgs[msgAddr{name, i}]
	if !ok {
		return nil, fmt.Errorf("fer: no such channel (name=%q index=%d)", name, i)
	}
	return msg, nil
}

func (dev *device) Done() chan Cmd {
	return dev.cmds
}

func (dev *device) isControler() {}

func (dev *device) Fatalf(format string, v ...interface{}) {
	dev.msg.Fatalf(format, v...)
}

func (dev *device) Printf(format string, v ...interface{}) {
	dev.msg.Printf(format, v...)
}

func (dev *device) run(ctx context.Context) {
	for n, chans := range dev.chans {
		dev.msg.Printf("--- init channels [%s]...\n", n)
		for i, ch := range chans {
			dev.msg.Printf("--- init channel[%s][%d]...\n", n, i)
			sck := ch.cfg.Sockets[0]
			switch strings.ToLower(sck.Method) {
			case "bind":
				go func() {
					err := ch.sck.Listen(sck.Address)
					if err != nil {
						dev.msg.Fatalf("listen(%q) error: %v\n", sck.Address, err)
					}
				}()
			case "connect":
				go func() {
					err := ch.sck.Dial(sck.Address)
					if err != nil {
						dev.msg.Fatalf("dial(%q) error: %v\n", sck.Address, err)
					}
				}()
			default:
				dev.msg.Fatalf("fer: invalid socket method (value=%q)", sck.Method)
			}
		}
	}

	for n, chans := range dev.chans {
		dev.msg.Printf("--- start channels [%s]...\n", n)
		for i := range chans {
			go chans[i].run(ctx)
		}
	}

	/*
		select {
		case <-ctx.Done():
			for n, chans := range dev.chans {
				dev.msg.Printf("--- stop channels [%s]...\n", n)
				for i := range chans {
					go func(i int) {
						chans[i].cmd <- CmdEnd
					}(i)
				}
			}
		}
	*/
}

// Device is a handle to what users get to run via the Fer toolkit.
//
// Devices are configured according to command-line flags and a JSON
// configuration file.
// Clients should implement the Run method to receive and send data via
// the Controler data channels.
type Device interface {
	// Configure hands a device its configuration.
	Configure(cfg config.Device) error
	// Init gives a chance to the device to initialize internal
	// data structures, retrieve channels to input/output data.
	Init(ctl Controler) error
	// Run is where the device's main activity happens.
	// Run should loop forever, until the Controler.Done() channel says
	// otherwise.
	Run(ctl Controler) error
	// Pause pauses the device's execution.
	Pause(ctl Controler) error
	// Reset resets the device's internal state.
	Reset(ctl Controler) error
}

// Controler controls devices execution and gives a device access to input and
// output data channels.
type Controler interface {
	Logger
	Chan(name string, i int) (chan Msg, error)
	Done() chan Cmd

	isControler()
}

// Logger gives access to printf-like facilities
type Logger interface {
	Fatalf(format string, v ...interface{})
	Printf(format string, v ...interface{})
}

type msgAddr struct {
	name string
	id   int
}

// Msg is a quantum of data being exchanged between devices.
type Msg struct {
	Data []byte // Data is the message payload.
	Err  error  // Err indicates whether an error occured.
}

// Main configures and runs a device's execution, managing its state.
func Main(dev Device) error {
	cfg, err := config.Parse()
	if err != nil {
		return err
	}

	return runDevice(context.Background(), cfg, dev)
}

func runDevice(ctx context.Context, cfg config.Config, dev Device) error {
	drv, err := mq.Open(cfg.Transport)
	if err != nil {
		return err
	}

	devName := cfg.ID
	devCfg, ok := cfg.Options.Device(devName)
	if !ok {
		return fmt.Errorf("fer: no such device %q", devName)
	}

	sys, err := newDevice(drv, devCfg)
	if err != nil {
		return err
	}

	err = dev.Configure(devCfg)
	if err != nil {
		return err
	}

	go sys.run(ctx)

	err = dev.Init(sys)
	if err != nil {
		return err
	}

	err = dev.Run(sys)
	if err != nil {
		return err
	}

	return nil
}
