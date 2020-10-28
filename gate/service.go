package gate

import (
	"flag"
	"fmt"

	"strings"

	"github.com/topfreegames/pitaya"
	"github.com/topfreegames/pitaya/acceptor"
	"github.com/topfreegames/pitaya/component"
	"github.com/topfreegames/pitaya/examples/demo/cluster_protobuf/services"
	"github.com/topfreegames/pitaya/serialize/protobuf"
)

func configureFrontend(port int) {
	ws := acceptor.NewWSAcceptor(fmt.Sprintf(":%d", port))
	pitaya.Register(&services.Connector{},
		component.WithName("connector"),
		component.WithNameFunc(strings.ToLower),
	)
	pitaya.RegisterRemote(&services.ConnectorRemote{},
		component.WithName("connectorremote"),
		component.WithNameFunc(strings.ToLower),
	)

	pitaya.AddAcceptor(ws)
}

func Start() {
	port := flag.Int("port", 3250, "the port to listen")
	svType := flag.String("type", "connector", "the server type")

	flag.Parse()

	defer pitaya.Shutdown()

	ser := protobuf.NewSerializer()

	pitaya.SetSerializer(ser)

	configureFrontend(*port)

	pitaya.Configure(true, *svType, pitaya.Cluster, map[string]string{})
	pitaya.Start()
}
