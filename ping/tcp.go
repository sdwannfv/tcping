package ping

import (
	"fmt"
	"net"
	"time"
)

// TCPing ...
type TCPing struct {
	target       *Target
	done         chan struct{}
	result       *Result
	packetResult chan packetResult
}
type packetResult struct {
	delay time.Duration
	addr  net.Addr
	err   error
}

var _ Pinger = (*TCPing)(nil)

// NewTCPing return a new TCPing
func NewTCPing() *TCPing {
	tcping := TCPing{
		done:         make(chan struct{}),
		packetResult: make(chan packetResult),
	}
	return &tcping
}

// SetTarget set target for TCPing
func (tcping *TCPing) SetTarget(target *Target) {
	tcping.target = target
	if tcping.result == nil {
		tcping.result = &Result{Target: target}
	}
}

// Result return the result
func (tcping TCPing) Result() *Result {
	return tcping.result
}

// Start a tcping
func (tcping TCPing) Start() <-chan struct{} {
	go func() {
		t := time.NewTicker(tcping.target.Interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if tcping.result.Counter >= tcping.target.Counter && tcping.target.Counter != 0 {
					tcping.Stop()
					return
				}
				go tcping.ping()
				tcping.result.Counter++

			case packetResult := <-tcping.packetResult:
				tcping.result.Counter++
				err := packetResult.err
				remoteAddr := packetResult.addr
				duration := packetResult.delay
				if packetResult.err != nil {
					fmt.Printf("Ping %s - failed: %s\n", tcping.target, err)
				} else {
					fmt.Printf("Ping %s(%s) - Connected - time=%s\n", tcping.target, remoteAddr, duration)

					if tcping.result.MinDuration == 0 {
						tcping.result.MinDuration = duration
					}
					if tcping.result.MaxDuration == 0 {
						tcping.result.MaxDuration = duration
					}
					tcping.result.SuccessCounter++
					if duration > tcping.result.MaxDuration {
						tcping.result.MaxDuration = duration
					} else if duration < tcping.result.MinDuration {
						tcping.result.MinDuration = duration
					}
					tcping.result.TotalDuration += duration
				}
			case <-tcping.done:
				return
			}
		}
	}()
	return tcping.done
}

// Stop the tcping
func (tcping *TCPing) Stop() {
	tcping.done <- struct{}{}
}

func (tcping TCPing) ping() {
	var remoteAddr net.Addr
	duration, errIfce := timeIt(func() interface{} {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", tcping.target.Host, tcping.target.Port), tcping.target.Timeout)
		if err != nil {
			return err
		}
		remoteAddr = conn.RemoteAddr()
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.SetLinger(0)
		}
		conn.Close()
		return nil
	})
	if errIfce != nil {
		err := errIfce.(error)
		tcping.packetResult <- packetResult{0, remoteAddr, err}
		return
	}

	tcping.packetResult <- packetResult{time.Duration(duration), remoteAddr, nil}
}
