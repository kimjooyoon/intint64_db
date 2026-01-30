package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"intint64_db/pkg/client"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <address> <port>\n  e.g. %s 127.0.0.1 7770\n", os.Args[0], os.Args[0])
		os.Exit(1)
	}
	address := os.Args[1]
	port := os.Args[2]
	addr := address + ":" + port

	c, err := client.New(addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer c.Close()

	fmt.Fprintf(os.Stderr, "connected to %s (type a.b.c.d and enter, e.g. 0.0.0.100 or 1.0.0.0)\n", addr)
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p, err := parsePacket(line)
		if err != nil {
			fmt.Fprintln(os.Stderr, "parse:", err)
			continue
		}
		if p[0] == 0 {
			if err := c.Send(p); err != nil {
				fmt.Fprintln(os.Stderr, "send:", err)
				continue
			}
		} else if p[0] == 6 {
			vals, err := c.Range(p[2], p[3])
			if err != nil {
				fmt.Fprintln(os.Stderr, "range:", err)
				continue
			}
			for _, v := range vals {
				fmt.Println(v)
			}
		} else {
			resp, err := c.Query(p)
			if err != nil {
				fmt.Fprintln(os.Stderr, "query:", err)
				continue
			}
			fmt.Println(resp[3])
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parsePacket(s string) (client.Packet, error) {
	var p client.Packet
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return p, fmt.Errorf("need 4 numbers (a.b.c.d), got %d", len(parts))
	}
	for i := range 4 {
		n, err := strconv.ParseInt(strings.TrimSpace(parts[i]), 10, 64)
		if err != nil {
			return p, err
		}
		p[i] = n
	}
	return p, nil
}
