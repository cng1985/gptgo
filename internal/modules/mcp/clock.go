package mcp

import "time"

var nowFn = time.Now

func clockNow() time.Time { return nowFn() }
