package network

import "regexp"

var protocols = map[uint32]string{
	1:  "SSH",
	2:  "TLS",
	3:  "HTTP",
	4:  "WS",
	5:  "SMTP",
	6:  "POP3",
	7:  "IMAP",
	8:  "FTP",
	9:  "RDP",
	10: "VNC",
	11: "Telnet",
	12: "Redis",
	13: "Postgre",
	14: "MySQL",
	15: "MongoDB",
	16: "MQTT",
	17: "AMQP",
	18: "SIP",
	19: "SOCKS5",
	20: "Steam",
	21: "RTMP",
}

var protocolMatchers = map[uint32]*regexp.Regexp{
	1:  regexp.MustCompile(`^SSH-\d+\.\d+-`),
	2:  regexp.MustCompile(`^\x16\x03[\x00-\x03]`),
	3:  regexp.MustCompile(`^(GET|POST|HEAD|PUT|DELETE|OPTIONS|TRACE|CONNECT) `),
	4:  regexp.MustCompile(`(?i)^GET .*Upgrade:\s*websocket`),
	5:  regexp.MustCompile(`^220 .* ESMTP`),
	6:  regexp.MustCompile(`^\+OK POP3`),
	7:  regexp.MustCompile(`^\* OK \[CAPABILITY`),
	8:  regexp.MustCompile(`^220 .* FTP server`),
	9:  regexp.MustCompile(`^\x03\x00\x00\x13\x0e\xd0`),
	10: regexp.MustCompile(`^RFB \d{3}\.\d{3}\n`),
	11: regexp.MustCompile(`^Trying .*\nConnected to`),
	12: regexp.MustCompile(`^\*1\r\n\$4\r\nPING\r\n`),
	13: regexp.MustCompile(`^\x00\x03\x00\x00`),
	14: regexp.MustCompile(`^\x10\x00\x00\x01`),
	15: regexp.MustCompile(`^\x80\x00\x00\x00`),
	16: regexp.MustCompile(`^\x10.`),
	17: regexp.MustCompile(`^AMQP\x00\x00\x09\x01`),
	18: regexp.MustCompile(`(?i)^(INVITE|REGISTER) sip:`),
	19: regexp.MustCompile(`^\x05\x01\x00`),
	20: regexp.MustCompile(`^ÿÿÿÿ`),
	// 21: regexp.MustCompile(`^\x03.{1536}`),
}

func detectProtocol(data []byte) uint32 {
	for v, matcher := range protocolMatchers {
		if matcher.Match(data) {
			return v
		}
	}

	return 0
}
