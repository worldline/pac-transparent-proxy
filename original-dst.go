package main

import (
	"net"
	"os"
	"unsafe"

	syscall "golang.org/x/sys/unix"
)

const soOriginalDst = 80

func getDestinationHost(conn *net.TCPConn) (string, int, error) {
	file, err := conn.File()
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	localIP, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return "", 0, err
	}
	if net.ParseIP(localIP).To4() != nil {
		return getDestinationHostIPV4(conn, file)
	}
	return getDestinationHostIPV6(conn, file)
}

func getsockopt(s uintptr, level, name int, v unsafe.Pointer, l *uint32) error {
	if _, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, s, uintptr(level), uintptr(name), uintptr(v), uintptr(unsafe.Pointer(l)), 0); errno != 0 {
		return error(errno)
	}
	return nil
}

func getDestinationHostIPV4(conn *net.TCPConn, file *os.File) (string, int, error) {
	var rsa syscall.RawSockaddrAny
	rsalen := uint32(syscall.SizeofSockaddrAny)
	err := getsockopt(file.Fd(), syscall.IPPROTO_IP, soOriginalDst, unsafe.Pointer(&rsa), &rsalen)
	if err != nil {
		return "", 0, err
	}

	// Parse getsockopt response
	pp := (*syscall.RawSockaddrInet4)(unsafe.Pointer(&rsa))
	addr := pp.Addr[0:net.IPv4len]
	p := (*[2]byte)(unsafe.Pointer(&pp.Port))
	port := int(p[0])<<8 + int(p[1])
	return net.IP(addr).String(), port, nil
}

func getDestinationHostIPV6(conn *net.TCPConn, file *os.File) (string, int, error) {
	var rsa syscall.RawSockaddrAny
	rsalen := uint32(syscall.SizeofSockaddrAny)
	err := getsockopt(file.Fd(), syscall.IPPROTO_IPV6, soOriginalDst, unsafe.Pointer(&rsa), &rsalen)
	if err != nil {
		return "", 0, err
	}

	// Parse getsockopt response
	pp := (*syscall.RawSockaddrInet6)(unsafe.Pointer(&rsa))
	addr := pp.Addr[0:net.IPv6len]
	p := (*[2]byte)(unsafe.Pointer(&pp.Port))
	port := int(p[0])<<8 + int(p[1])
	return net.IP(addr).String(), port, nil
}
