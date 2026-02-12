package tailnet

import "monks.co/pkg/requireenv"

var tailscaleAuthKey = requireenv.Require("TS_AUTHKEY")
