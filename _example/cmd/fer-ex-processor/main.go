// Copyright 2016 The fer Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"time"

	"github.com/alice-go/fer"
	"github.com/alice-go/fer/config"
)

type processor struct {
	cfg    config.Device
	idatac chan fer.Msg
	odatac chan fer.Msg
}

func (dev *processor) Configure(cfg config.Device) error {
	dev.cfg = cfg
	return nil
}

func (dev *processor) Init(ctl fer.Controller) error {
	idatac, err := ctl.Chan("data1", 0)
	if err != nil {
		return err
	}

	odatac, err := ctl.Chan("data2", 0)
	if err != nil {
		return err
	}

	dev.idatac = idatac
	dev.odatac = odatac
	return nil
}

func (dev *processor) Run(ctl fer.Controller) error {
	for {
		select {
		case data := <-dev.idatac:
			// ctl.Printf("received: %q\n", string(data.Data))
			out := append([]byte(nil), data.Data...)
			out = append(out, []byte(" (modified by "+dev.cfg.Name()+")"+time.Now().String())...)
			dev.odatac <- fer.Msg{Data: out}
		case <-ctl.Done():
			return nil
		}
	}
}

func main() {
	err := fer.Main(&processor{})
	if err != nil {
		log.Fatal(err)
	}
}
