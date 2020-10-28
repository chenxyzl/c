# 数据库 proto文件书写规范
  
## 说明：
(db_proto.md common.proto)    
`每个新的大版本要对前版本做数据向前兼容，开发期不要求做兼容（开发期数据结构变动太频繁）`

## 基本规则：
1. 每个proto类型必须有注释，每个字段也必须有注释。注释格式和字段命名规范参照下面
    ```
    // ServerMail 全局邮件内容
    message ServerMail {
      uint64 mid = 1; //邮件id，无意义，只是用来保存在db的key不重复
      int32 mailTId = 2; //邮件类型id
      int64 receiveTime = 3; //邮件接收时间
      string title = 4; //标题
      string content = 5; //内容
      repeated string paramds = 6; //参数
      repeated common.Item attachment = 7; //附件
      uint64 moduleParam = 8; //自定义参数 (例如竞技场的versionId)
      repeated common.MailCondition mailConditions = 9; //邮件条件（主要来自gm）
    }
    ```
    
1. 大的功能模块区域划分
    ```
    //--------------------------------------------------------------------------------------------------特权
    ```
1. 字段id可以按照一定逻辑分段
    ```
    //UserGameDB 用户子数据
    message UserGameDB {
    
      // 基础
      Bag bag = 1; //背包
      Vip vip = 2; //Vip
      Recharge recharge = 3; //充值
      Chat chat = 4; //聊天数据
      Stats stats = 5; //统计数据
      UserMail mailDB = 6; //玩家邮件
      UserSetting setting = 7; //设置
    
      // 战斗
      CounsellorGroup CounsellorGroup = 20; //军师
      map<int32, Commander> commanderDB = 21; //英雄
      map<int32, common.Formation> formationDB = 22; //阵型数据
      Soldier soldier = 23; //士兵
    
      // 养成
      HandBook handbook = 30; //领主手册
      Treasure treasure = 31; //宝物
    }
    ```

## 向前兼容规则：
1. 每个字段的id和类型绝对不能改
1. 新版本中如果有字段无用了，一定要注释掉，而不是删除掉（目的是为了保留id，不会被别人占用）
1. 如果新版本新增字段的初始值需要由老版本数据转化，则要增加DB兼容函数