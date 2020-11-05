# c
go 游戏服务器框架
demo

#gate
对外链接 tcp/ws/udp 并维护心跳
#lobby
 分布式玩家业务处理[登陆后随机分配一个lobby，并在etcd上注册session]
#server
 服务器玩法处理[分服或不分服取决于怎么收集lobby上的玩家数据]
#big_world
 集群玩法-->无限大世界[示例-如联盟等]
#admin[web服务器]
 管理集群状态，服务器状态等，服务器配置等
#login[web服务]
 服务器列表,创建账号等[客户端获取服务器列表，状态等，创建角色等]
 
#etcd
 注册服务-运行状态数据
 服务器配置获取
 保持玩家session，服务下线则该服务下的session无效
 策划配置表
 gm活动，运维数据
#nats
 内部rpc消息队列
#elk/tidb
 内部日志消费/数据收集和分析
 
#启动集群流程
1.启动mongodb/etcd/nats [可选日志打点kafka/elk/tidb]
2.启动admin，配置集群参数，服务器参数
3.启动login/server/big_world等业务服务器
last.客户端在login登陆后，获取服务器列表，链接gate，选服务器发送进入请求
#客户端进入游戏流程
1.login登陆
2.获取login的服务器列表
3.链接gate，发送进入游戏服务器命令