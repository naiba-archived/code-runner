package global

import "context"

var Context, CancelGlobal = context.WithCancel(context.Background())
