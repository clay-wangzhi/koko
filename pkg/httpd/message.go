package httpd

import (
	"time"

	"github.com/jumpserver/koko/pkg/jms-sdk-go/model"
)

type Message struct {
	Id   string `json:"id"`
	Type string `json:"type"`
	Data string `json:"data"`
	Raw  []byte `json:"-"`
	Err  string `json:"err"`
}

const (
	PING           = "PING"
	PONG           = "PONG"
	CONNECT        = "CONNECT"
	CLOSE          = "CLOSE"
	TERMINALINIT   = "TERMINAL_INIT"
	TERMINALDATA   = "TERMINAL_DATA"
	TERMINALRESIZE = "TERMINAL_RESIZE"
	TERMINALBINARY = "TERMINAL_BINARY"
	TERMINALACTION = "TERMINAL_ACTION"

	TERMINALSESSION = "TERMINAL_SESSION"

	TERMINALSHARE      = "TERMINAL_SHARE"
	TERMINALSHAREJOIN  = "TERMINAL_SHARE_JOIN"
	TERMINALSHARELEAVE = "TERMINAL_SHARE_LEAVE"
	TERMINALSHAREUSERS = "TERMINAL_SHARE_USERS"

	TERMINALERROR = "TERMINAL_ERROR"
)

type WindowSize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type TerminalConnectData struct {
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
	Code string `json:"code"`
}

type ShareRequestParams struct {
	SessionID  string `json:"session_id"`
	ExpireTime int    `json:"expired"`
}

type ShareResponse struct {
	ShareId string `json:"share_id"`
	Code    string `json:"code"`
}

type ShareInfo struct {
	Record model.ShareRecord
}

const (
	TargetTypeAsset = "asset"

	// TargetTypeMonitor todo: 前端参数将 统一修改成 monitor
	TargetTypeMonitor = "shareroom"

	TargetTypeShare = "share"
)

const (
	maxReadTimeout  = 5 * time.Minute
	maxWriteTimeOut = 5 * time.Minute
)

const (
	TTYName       = "terminal"
	WebFolderName = "web_folder"
)

type ViewPageMata struct {
	ID      string
	IconURL string
}
