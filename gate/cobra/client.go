package cobra

import "time"

//启动 tcp client
func TCPClientServe(se Sessioner, conf *Config, timeout time.Duration) bool {
	return newBroker(se, conf).Connect(timeout)
}

//启动 websocket client
func WSClientServe(se Sessioner, conf *Config, timeout time.Duration) bool {
	return newBroker(se, conf).WsConnect()
}
