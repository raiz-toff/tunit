package net

import "testing"

func TestParsePingOutputLinux(t *testing.T) {
	output := `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.

--- 8.8.8.8 ping statistics ---
10 packets transmitted, 10 received, 0% packet loss, time 9013ms
rtt min/avg/max/mdev = 4.123/6.456/9.789/1.234 ms
`
	stats := parsePingOutput(output)

	if stats.min < 4.0 || stats.min > 4.2 {
		t.Errorf("min = %f, want ~4.123", stats.min)
	}
	if stats.avg < 6.4 || stats.avg > 6.5 {
		t.Errorf("avg = %f, want ~6.456", stats.avg)
	}
	if stats.max < 9.7 || stats.max > 9.8 {
		t.Errorf("max = %f, want ~9.789", stats.max)
	}
	if stats.loss != 0 {
		t.Errorf("loss = %f, want 0", stats.loss)
	}
}

func TestParsePingOutputWithLoss(t *testing.T) {
	output := `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.

--- 8.8.8.8 ping statistics ---
10 packets transmitted, 8 received, 20% packet loss, time 9013ms
rtt min/avg/max/mdev = 4.123/6.456/9.789/1.234 ms
`
	stats := parsePingOutput(output)
	if stats.loss != 20 {
		t.Errorf("loss = %f, want 20", stats.loss)
	}
}

func TestParsePingOutputWindows(t *testing.T) {
	output := `Pinging 8.8.8.8 with 32 bytes of data:

Ping statistics for 8.8.8.8:
    Packets: Sent = 4, Received = 4, Lost = 0 (0% loss),
Approximate round trip times in milli-seconds:
    Minimum = 4ms, Maximum = 9ms, Average = 6ms
`
	stats := parsePingOutput(output)
	if stats.min != 4 {
		t.Errorf("min = %f, want 4", stats.min)
	}
	if stats.avg != 6 {
		t.Errorf("avg = %f, want 6", stats.avg)
	}
	if stats.max != 9 {
		t.Errorf("max = %f, want 9", stats.max)
	}
}

func TestParsePingOutputEmpty(t *testing.T) {
	stats := parsePingOutput("")
	if stats.min != 0 || stats.avg != 0 || stats.max != 0 || stats.loss != 0 {
		t.Error("empty input should return zero stats")
	}
}

func TestParsePingOutput100Loss(t *testing.T) {
	output := `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.

--- 8.8.8.8 ping statistics ---
10 packets transmitted, 0 received, 100% packet loss, time 9013ms
`
	stats := parsePingOutput(output)
	if stats.loss != 100 {
		t.Errorf("loss = %f, want 100", stats.loss)
	}
}
