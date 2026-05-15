// Text terminal view for local clients.
package term

import (
	"fmt"
	"ngrok/client/mvc"
	"ngrok/log"
	"ngrok/proto"
)

type TermView struct {
	ctl      mvc.Controller
	updates  chan interface{}
	shutdown chan struct{}
	log.Logger
}

type HttpView struct {
	shutdown chan struct{}
	log.Logger
}

func NewTermView(ctl mvc.Controller) *TermView {
	v := &TermView{
		ctl:      ctl,
		updates:  ctl.Updates().Reg(),
		shutdown: make(chan struct{}),
		Logger:   log.NewPrefixLogger("view", "term"),
	}

	ctl.Go(v.run)
	return v
}

func connStatusRepr(status mvc.ConnStatus) string {
	switch status {
	case mvc.ConnConnecting:
		return "connecting"
	case mvc.ConnReconnecting:
		return "reconnecting"
	case mvc.ConnOnline:
		return "online"
	}
	return "unknown"
}

func (v *TermView) printState(state mvc.State) {
	fmt.Printf("Tunnel Status: %s\n", connStatusRepr(state.GetConnStatus()))
	fmt.Printf("Version: %s/%s\n", state.GetClientVersion(), state.GetServerVersion())
	for _, t := range state.GetTunnels() {
		fmt.Printf("Forwarding: %s -> %s\n", t.PublicUrl, t.LocalAddr)
	}
	fmt.Printf("Web Interface: %s\n", v.ctl.GetWebInspectAddr())
}

func (v *TermView) run() {
	defer v.ctl.Updates().UnReg(v.updates)
	v.printState(v.ctl.State())
	for {
		select {
		case state := <-v.updates:
			v.printState(state.(mvc.State))
		case <-v.shutdown:
			return
		}
	}
}

func (v *TermView) Shutdown() {
	close(v.shutdown)
}

func (v *TermView) NewHttpView(p *proto.Http) *HttpView {
	vh := &HttpView{
		shutdown: make(chan struct{}),
		Logger:   log.NewPrefixLogger("view", "term", "http"),
	}
	return vh
}

func (v *HttpView) Shutdown() {
	close(v.shutdown)
}
