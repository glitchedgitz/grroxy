package utils

import (
	"fmt"
	"log"
	"net"
	"strconv"
)

func ExtractHostPortFromURL(rawURL string) (string, int, error) {
	host, port, err := net.SplitHostPort(rawURL)
	if err != nil {
		return "", 0, err
	}

	// Convert port string to int
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, err
	}

	return host, portInt, nil
}

func IsPortAvailable(host string) bool {
	log.Println("Checking port: ", host)
	conn, err := net.Listen("tcp", host)
	// conn, err := net.DialTimeout("tcp", ":"+strconv.Itoa(port), time.Second*time.Duration(5))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func CheckAndFindAvailablePort(hostAddress string) (string, error) {

	host, port, err := ExtractHostPortFromURL(hostAddress)

	if err != nil {
		return "", fmt.Errorf("port not found in %s", hostAddress)
	}

	for start := port; start <= port+100; port++ {
		address := host + ":" + strconv.Itoa(port)
		if IsPortAvailable(address) {
			return address, nil
		}
	}

	// It's werid if it ever reached here
	return host + strconv.Itoa(port), fmt.Errorf("no available port in the range %d-%d", port, port+100)
}
