package tailnet

import "monks.co/pkg/requireenv"

var authKey = requireenv.Lazy("TS_AUTHKEY")
