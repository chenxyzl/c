package cobra

import (
	"github.com/topfreegames/pitaya/logger"
	"golang.org/x/net/websocket"
	"net"
	"net/http"
)

type Server interface {
	NewSession() Sessioner
}

//启动tcp server
func TCPServe(srv Server, conf *Config) {
	l, e := net.Listen("tcp", conf.Address)
	if e != nil {
		logger.Log.Error("[TCPServer] listen error: %v", e)
		panic(e.Error())
	}

	defer l.Close()

	for {
		rw, e := l.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				continue
			}
			logger.Log.Error("[TCPServer] accept error: %v", e)
			return
		}
		newBroker(srv.NewSession(), conf).serve(rw, ConnectType_TCP)
	}
}

//启动websocket server
func WsServe(srv Server, conf *NetConfig) {
	http.Handle(conf.Handler, websocket.Handler(func(conn *websocket.Conn) {
		conn.PayloadType = websocket.BinaryFrame
		newBroker(srv.NewSession(), &conf.Config).serve(conn, ConnectType_WS)

		////rbuf := bufio.NewReader(conn)
		//for {
		//	conn.SetReadDeadline(time.Now().Add(60000000000))
		//	data := make([]byte,1)
		//	_, err := conn.Read(data)
		//	if err != nil {
		//		logger.Log.Error(err)
		//	} else {
		//		//len := (uint32(data[0]) << 24) | (uint32(data[1]) << 16) | (uint32(data[2]) << 8) | uint32(data[3])
		//		logger.Log.Info(string(data))
		//		go func() {
		//			conn.Write([]byte("9876543210"))
		//		}()
		//	}
		//}
		//io.Copy(conn, conn)
	}))
	e := http.ListenAndServe(conf.WsAddress, nil)
	if e != nil {
		logger.Log.Error("[WebSocketServer] listen error: %v", e)
		panic(e.Error())
	}
}
