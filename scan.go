package main

import (
	"log"
	"net"
	"sync"
	"time"
)

var (
	mutex = &sync.Mutex{}
)

func getLocalADBAddresses() (out []string) {
	addresses := getLocalAddresses()
	for _, ip := range addresses {
		a, b, c := (*ip)[0], (*ip)[1], (*ip)[2]
		var wg sync.WaitGroup
		for i := 2; i < 256; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				addr := net.IPv4(a, b, c, byte(i))
				target := addr.String() + ":5555"
				if !hasPort(target) {
					return
				}
				log.Println("found", target)
				mutex.Lock()
				out = append(out, target)
				mutex.Unlock()
			}(i)
		}
		wg.Wait()
	}
	return
}

func getLocalAddresses() (addresses []*net.IP) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			addresses = append(addresses, &ip)
		}
	}
	return
}

func hasPort(target string) bool {
	conn, err := net.DialTimeout("tcp", target, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
