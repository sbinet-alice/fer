// Copyright 2018 The fer Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//+build czmq

package fer

import (
	_ "github.com/alice-go/fer/mq/czmq" // load C-bindings zeromq plugin
)
