package proxy

import (
	"net"
	"sync"
	"testing"

	"database_firewall/internal/config"
)

func TestConnectionLimits(t *testing.T) {
	ccfg := &config.ConnectionConfig{
		ConnectionLimit:      2,
		PerIPConnectionLimit: 1,
	}

	ac := &AdmissionController{
		RateLimiter: nil,
		ConnReg:     NewConnectionRegister(ccfg),
	}

	ips := []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2"), net.ParseIP("10.0.0.3")}

	//testing first connection -> expected to accept
	ok, _ := ac.Admit(ips[0])
	if !ok {
		t.Fatal("expected first connection to be allowed")
	}

	//Second connection from same ip -> expected to reject(per ip limit)
	ok, reason := ac.Admit(ips[0])
	if ok || reason != "per_ip_limit" {
		t.Fatalf("expected per_ip_limit rejection, got ok=%v reason=%v", ok, reason)
	}

	//Second connection from different ip -> expected to accept
	ok, _ = ac.Admit(ips[1])
	if !ok {
		t.Fatal("expected second connection from different ip to be allowed")
	}

	//Third connection from different ip -> expected to reject(connection limit)
	ok, reason = ac.Admit(ips[1])
	if ok || reason != "connection_limit" {
		t.Fatalf("expected connection_limit rejection, got ok=%v reason=%v", ok, reason)
	}
}

func TestAdmissionController_Concurrency(t *testing.T) {
	ccfg := &config.ConnectionConfig{
		ConnectionLimit:      10,
		PerIPConnectionLimit: 5,
	}

	connReg := NewConnectionRegister(ccfg)
	ac := AdmissionController{
		RateLimiter: nil,
		ConnReg:     connReg,
	}

	ip := net.ParseIP("10.0.0.1")

	const goroutines = 100
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ac.Admit(ip)

		}()
	}

	close(start)
	wg.Wait()

	active := connReg.ConnectionsByIP[ip.String()]
	if active > ccfg.PerIPConnectionLimit {
		t.Fatalf("invariant broken: active=%d > per_ip_limit=%d", active, ccfg.PerIPConnectionLimit)
	}
}
