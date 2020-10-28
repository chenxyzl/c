package lobby

import (
	"flag"
	"strings"

	"github.com/topfreegames/pitaya"
	"github.com/topfreegames/pitaya/component"
	"github.com/topfreegames/pitaya/examples/demo/cluster_protobuf/services"
	"github.com/topfreegames/pitaya/serialize/protobuf"
)

func configureBackend() {
	room := services.NewRoom()
	pitaya.Register(room,
		component.WithName("room"),
		component.WithNameFunc(strings.ToLower),
	)

	pitaya.RegisterRemote(room,
		component.WithName("room"),
		component.WithNameFunc(strings.ToLower),
	)
}

func Start() {
	svType := flag.String("type", "connector", "the server type")

	flag.Parse()

	defer pitaya.Shutdown()

	ser := protobuf.NewSerializer()

	pitaya.SetSerializer(ser)

	configureBackend()

	pitaya.Configure(false, *svType, pitaya.Cluster, map[string]string{})
	pitaya.Start()
}
