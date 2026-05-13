package scan

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type rtspResponse struct {
	status  int
	headers map[string]string
	body    []byte
}

type rtspClient struct {
	conn    net.Conn
	br      *bufio.Reader
	cseq    int
	udpConn *net.UDPConn // RTP data socket for UDP transport
}

func dialRTSP(host string, timeout time.Duration) (*rtspClient, error) {
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return nil, err
	}
	return &rtspClient{conn: conn, br: bufio.NewReader(conn)}, nil
}

func (c *rtspClient) close() {
	if c.udpConn != nil {
		c.udpConn.Close()
	}
	c.conn.Close()
}

// listenUDP opens a UDP socket on a random ephemeral port and stores it on
// the client. Returns the even RTP port number chosen by the OS.
func (c *rtspClient) listenUDP() (int, error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return 0, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	c.udpConn = conn
	port := conn.LocalAddr().(*net.UDPAddr).Port
	// RTP convention: RTP on even port, RTCP on odd port.
	// If the OS gave us an odd port, close and try to get an even one.
	if port%2 != 0 {
		conn.Close()
		// Try a few times to get an even port.
		for i := 0; i < 10; i++ {
			conn2, err2 := net.ListenUDP("udp", addr)
			if err2 != nil {
				return 0, err2
			}
			port = conn2.LocalAddr().(*net.UDPAddr).Port
			if port%2 == 0 {
				c.udpConn = conn2
				return port, nil
			}
			conn2.Close()
		}
		// Fall back to whatever we get — most devices don't care about even/odd.
		conn3, err3 := net.ListenUDP("udp", addr)
		if err3 != nil {
			return 0, err3
		}
		c.udpConn = conn3
		port = conn3.LocalAddr().(*net.UDPAddr).Port
	}
	return port, nil
}

// readUDPPacket reads a single RTP packet from the UDP socket.
func (c *rtspClient) readUDPPacket() ([]byte, error) {
	buf := make([]byte, 2048)
	n, err := c.udpConn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (c *rtspClient) teardown(controlURL, session string) {
	c.cseq++
	req := fmt.Sprintf("TEARDOWN %s RTSP/1.0\r\nCSeq: %d\r\nUser-Agent: dvbscan\r\nSession: %s\r\n\r\n",
		controlURL, c.cseq, session)
	c.conn.Write([]byte(req)) //nolint
}

func (c *rtspClient) send(method, url string, extra map[string]string, body []byte) (*rtspResponse, error) {
	c.cseq++
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s RTSP/1.0\r\nCSeq: %d\r\nUser-Agent: dvbscan\r\n", method, url, c.cseq)
	for k, v := range extra {
		fmt.Fprintf(&sb, "%s: %s\r\n", k, v)
	}
	if len(body) > 0 {
		fmt.Fprintf(&sb, "Content-Length: %d\r\n", len(body))
	}
	sb.WriteString("\r\n")
	if _, err := c.conn.Write([]byte(sb.String())); err != nil {
		return nil, err
	}
	if len(body) > 0 {
		if _, err := c.conn.Write(body); err != nil {
			return nil, err
		}
	}
	return c.readResponse()
}

func (c *rtspClient) readResponse() (*rtspResponse, error) {
	line, err := c.br.ReadString('\n')
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("bad status line: %q", line)
	}
	status, _ := strconv.Atoi(parts[1])
	hdrs := map[string]string{}
	for {
		line, err = c.br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if kv := strings.SplitN(line, ":", 2); len(kv) == 2 {
			hdrs[strings.ToLower(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
		}
	}
	var body []byte
	if cl, ok := hdrs["content-length"]; ok {
		n, _ := strconv.Atoi(cl)
		if n > 0 {
			body = make([]byte, n)
			if _, err = io.ReadFull(c.br, body); err != nil {
				return nil, err
			}
		}
	}
	return &rtspResponse{status: status, headers: hdrs, body: body}, nil
}

func (c *rtspClient) readInterleaved() ([]byte, error) {
	for {
		b, err := c.br.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != '$' {
			continue
		}
		ch, err := c.br.ReadByte()
		if err != nil {
			return nil, err
		}
		var lenBuf [2]byte
		if _, err = io.ReadFull(c.br, lenBuf[:]); err != nil {
			return nil, err
		}
		length := binary.BigEndian.Uint16(lenBuf[:])
		data := make([]byte, length)
		if _, err = io.ReadFull(c.br, data); err != nil {
			return nil, err
		}
		if ch == 0 {
			return data, nil
		}
	}
}

func stripRTPHeader(pkt []byte) ([]byte, error) {
	if len(pkt) < 12 {
		return nil, fmt.Errorf("RTP packet too short")
	}
	cc := int(pkt[0] & 0x0f)
	offset := 12 + cc*4
	if pkt[0]&0x10 != 0 {
		if len(pkt) < offset+4 {
			return nil, fmt.Errorf("RTP extension header too short")
		}
		extLen := int(binary.BigEndian.Uint16(pkt[offset+2:])) * 4
		offset += 4 + extLen
	}
	if offset > len(pkt) {
		return nil, fmt.Errorf("RTP header overruns packet")
	}
	return pkt[offset:], nil
}
